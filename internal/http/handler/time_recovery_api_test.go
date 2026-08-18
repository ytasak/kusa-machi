package handler_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"kusamachi/internal/apptest"
	"kusamachi/internal/clock"
	"kusamachi/internal/matching"
)

// recoveryInterval はサーバ側の回復間隔。テストはこの定数を進める。
const recoveryInterval = matching.TimeRecoveryInterval

// homeRaw は Client の状態に触らずにホームを読む。並行アクセスの検証で使う。
func homeRaw(t *testing.T, c *apptest.Client) apptest.HomeResponse {
	t.Helper()
	res := c.Do(t, http.MethodGet, "/api/home", nil)
	res.RequireStatus(t, http.StatusOK)

	var home apptest.HomeResponse
	res.Decode(t, &home)
	return home
}

// requireNextRecovery は次回回復の時刻が JST の期待した壁時計時刻かを確かめる。
func requireNextRecovery(t *testing.T, got *string, wantHour, wantMin int) {
	t.Helper()
	if got == nil {
		t.Fatalf("next_recovery_at = null, want %02d:%02d", wantHour, wantMin)
	}
	parsed, err := time.Parse(time.RFC3339, *got)
	if err != nil {
		t.Fatalf("next_recovery_at %q is not RFC3339: %v", *got, err)
	}
	if h, m, _ := parsed.Clock(); h != wantHour || m != wantMin {
		t.Fatalf("next_recovery_at = %s, want %02d:%02d JST", *got, wantHour, wantMin)
	}
}

// requireAnchor は回復の起点が JST の期待した壁時計時刻かを確かめる。
// 上限で1つも増えなかった3時間も起点を進めることの検証に使う。
func requireAnchor(t *testing.T, app *apptest.App, personaID string, wantHour, wantMin int) {
	t.Helper()
	anchor := app.RecoveryAnchor(t, personaID)
	if anchor == nil {
		t.Fatalf("like_recovery_anchor_at = null, want %02d:%02d", wantHour, wantMin)
	}
	if h, m, _ := anchor.In(clock.JST).Clock(); h != wantHour || m != wantMin {
		t.Fatalf("like_recovery_anchor_at = %s, want %02d:%02d JST", anchor, wantHour, wantMin)
	}
}

func TestTheTimerStaysStoppedUntilTheFirstLikeIsSpent(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	// Like を使わずに何時間放置しても、初期の10に上積みはされない。
	app.Clock.Advance(3 * recoveryInterval)

	home := alice.Home(t)
	if home.RemainingLikes != matching.DailyLikeBudget {
		t.Fatalf("remaining_likes = %d, want the plain budget %d", home.RemainingLikes, matching.DailyLikeBudget)
	}
	if home.LikeCapacity != matching.LikeCap {
		t.Fatalf("like_capacity = %d, want %d", home.LikeCapacity, matching.LikeCap)
	}
	if home.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0", home.LikesRecovered)
	}
	if home.NextRecoveryAt != nil {
		t.Fatalf("next_recovery_at = %s, want null before the first like", *home.NextRecoveryAt)
	}

	if state := app.RewardState(t, aliceCard.ID); state.RecoveryAnchorSet {
		t.Fatalf("reward state = %+v, want a stopped timer", state)
	}
}

func TestTheFirstLikeStartsTheTimer(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	_, targets := app.NewStartedClients(t, 2)

	// 時計は 12:00 に固定されている。1つ目の Like がその時刻を起点にする。
	res := alice.MustLike(t, targets[0].ID)
	requireNextRecovery(t, res.NextRecoveryAt, 15, 0)

	if state := app.RewardState(t, aliceCard.ID); !state.RecoveryAnchorSet {
		t.Fatalf("reward state = %+v, want the timer started", state)
	}

	// 2つ目の Like で起点が今に進んでしまうと、使うほど回復が遠のく。
	app.Clock.Advance(time.Hour)
	second := alice.MustLike(t, targets[1].ID)
	requireNextRecovery(t, second.NextRecoveryAt, 15, 0)
}

func TestNothingRecoversBeforeTheInterval(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 1)
	app.Clock.Advance(recoveryInterval - time.Minute)

	home := alice.Home(t)
	if want := matching.DailyLikeBudget - 1; home.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, want)
	}
	if home.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0 before the interval", home.LikesRecovered)
	}
}

func TestOneLikeRecoversPerInterval(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 3)
	app.Clock.Advance(recoveryInterval)

	// ホームを開いた時点で経過時間から計算される。Cron もジョブも要らない。
	home := alice.Home(t)
	if want := matching.DailyLikeBudget - 3 + 1; home.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, want)
	}
	if home.LikesRecovered != 1 {
		t.Fatalf("likes_recovered = %d, want 1", home.LikesRecovered)
	}
	// 起点が3時間進んだので、次は 18:00。
	requireNextRecovery(t, home.NextRecoveryAt, 18, 0)
	requireAnchor(t, app, aliceCard.ID, 15, 0)
}

// アプリを閉じていた時間も回復時間として経過する。次に開いた時点で、
// 経過した3時間単位をまとめて計算する。
func TestTimeAwayFromTheAppRecoversAllAtOnce(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 3)

	// 3時間ごとに見に来る必要はない。9時間半ぶん閉じていた場合、
	// floor(9.5h / 3h) = 3 回ぶんがまとめて成立する。
	app.Clock.Advance(3*recoveryInterval + 30*time.Minute)

	home := alice.Home(t)
	if home.LikesRecovered != 3 {
		t.Fatalf("likes_recovered = %d, want 3", home.LikesRecovered)
	}
	if want := matching.DailyLikeBudget - 3 + 3; home.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, want)
	}
	// 起点は 12:00 から3回ぶん進んで 21:00。余った30分は次回へ持ち越される。
	requireAnchor(t, app, aliceCard.ID, 21, 0)
}

// 時間回復の回数に1日の上限は無い。同じ日のうちに3回以上続けて起きる。
func TestTimeRecoveryHasNoDailyLimit(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 5)

	for i := 1; i <= 3; i++ {
		app.Clock.Advance(recoveryInterval)

		home := alice.Home(t)
		if home.LikesRecovered != 1 {
			t.Fatalf("recovery %d: likes_recovered = %d, want 1", i, home.LikesRecovered)
		}
		if want := matching.DailyLikeBudget - 5 + i; home.RemainingLikes != want {
			t.Fatalf("recovery %d: remaining_likes = %d, want %d", i, home.RemainingLikes, want)
		}
	}
}

// 所持上限を超える回復は失われる。後から Like を使っても戻ってこない。
func TestRecoveryBeyondTheCapIsLost(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)

	// Match 1回で 10 - 1 + 2 = 11。起点は最初の Like の 12:00。
	mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID)
	if state := app.RewardState(t, aliceCard.ID); state.LikeBalance != matching.LikeCap-1 {
		t.Fatalf("like_balance = %d, want %d", state.LikeBalance, matching.LikeCap-1)
	}

	// 3回ぶんの時間が経っても、上限まで1つしか空いていないので増えるのは1つ。
	app.Clock.Advance(3 * recoveryInterval)

	home := alice.Home(t)
	if home.LikesRecovered != 1 {
		t.Fatalf("likes_recovered = %d, want 1 — the cap clips the rest", home.LikesRecovered)
	}
	if home.RemainingLikes != matching.LikeCap {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, matching.LikeCap)
	}
	// 満タンなので待つ意味が無く、タイマーも出さない。
	if home.NextRecoveryAt != nil {
		t.Fatalf("next_recovery_at = %s, want null at the cap", *home.NextRecoveryAt)
	}
	// 起点は受け取れなかった2回ぶんも含めて3回ぶん進んでいる。
	requireAnchor(t, app, aliceCard.ID, 21, 0)

	// Like を2つ使って空きを作っても、失われた2回ぶんは戻らない。
	app.SpendLikes(t, alice, 2)
	after := alice.Home(t)
	if after.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0 — the lost recoveries must not come back", after.LikesRecovered)
	}
	if want := matching.LikeCap - 2; after.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", after.RemainingLikes, want)
	}
}

// 満タンのまま3時間が過ぎても、回復はストックされない。空きができた時点から
// 通常の間隔で回復し直す。
func TestAFullBalanceDoesNotStockRecoveries(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)
	carol, carolCard := app.NewStartedClient(t)

	// Match 2回で 10 - 2 + 4 = 12。所持上限にちょうど届く。
	mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID)
	mutualLike(t, alice, aliceCard.ID, carol, carolCard.ID)
	if state := app.RewardState(t, aliceCard.ID); state.LikeBalance != matching.LikeCap {
		t.Fatalf("like_balance = %d, want the cap %d", state.LikeBalance, matching.LikeCap)
	}

	// 満タンのまま2回ぶん経過する。何も増えないが、起点は進む。
	app.Clock.Advance(2 * recoveryInterval)
	home := alice.Home(t)
	if home.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0 at the cap", home.LikesRecovered)
	}
	if home.RemainingLikes != matching.LikeCap {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, matching.LikeCap)
	}
	requireAnchor(t, app, aliceCard.ID, 18, 0)

	// 1つ使って空きを作る。ストックしていれば、ここで一気に戻ってしまう。
	app.SpendLikes(t, alice, 1)
	spent := alice.Home(t)
	if spent.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0 — the full hours must not be stocked", spent.LikesRecovered)
	}
	if want := matching.LikeCap - 1; spent.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", spent.RemainingLikes, want)
	}
	// 次の回復は通常の間隔どおり 21:00。
	requireNextRecovery(t, spent.NextRecoveryAt, 21, 0)

	app.Clock.Advance(recoveryInterval)
	if next := alice.Home(t); next.LikesRecovered != 1 || next.RemainingLikes != matching.LikeCap {
		t.Fatalf("after the next interval likes_recovered = %d, remaining_likes = %d, want 1 and %d",
			next.LikesRecovered, next.RemainingLikes, matching.LikeCap)
	}
}

func TestConcurrentAccessGrantsTheRecoveryOnlyOnce(t *testing.T) {
	const tabs = 6

	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 4)
	app.Clock.Advance(recoveryInterval)

	// 複数のタブが同時にホームを開く。回復は1つだけ付与されなければならない。
	var (
		mu    sync.Mutex
		total int
		wg    sync.WaitGroup
	)
	for range tabs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			home := homeRaw(t, alice)
			mu.Lock()
			total += home.LikesRecovered
			mu.Unlock()
		}()
	}
	wg.Wait()

	if total != 1 {
		t.Fatalf("total likes_recovered across %d tabs = %d, want 1", tabs, total)
	}

	state := app.RewardState(t, aliceCard.ID)
	if want := matching.DailyLikeBudget - 4 + 1; state.LikeBalance != want {
		t.Fatalf("like_balance = %d, want %d", state.LikeBalance, want)
	}
	requireAnchor(t, app, aliceCard.ID, 15, 0)
}

// GET で反映した回復を、続く POST がもう一度適用しない。
func TestARecoveryAppliedByAGetIsNotReappliedByAPost(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)
	_, targets := app.NewStartedClients(t, 1)

	app.SpendLikes(t, alice, 3)
	app.Clock.Advance(recoveryInterval)

	if home := alice.Home(t); home.LikesRecovered != 1 {
		t.Fatalf("likes_recovered = %d, want 1 on the first read", home.LikesRecovered)
	}

	res := alice.MustLike(t, targets[0].ID)
	if res.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0 — the same interval must not pay twice", res.LikesRecovered)
	}
	if want := matching.DailyLikeBudget - 3 + 1 - 1; res.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", res.RemainingLikes, want)
	}
}

func TestARecoveredLikeIsSpendableImmediately(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)

	// 使い切ってから3時間待つ。残数0のまま開いていた画面から送っても、
	// POST の中で回復が反映されるので1つ送れる。
	app.SpendLikes(t, alice, matching.DailyLikeBudget)
	if home := alice.Home(t); home.RemainingLikes != 0 {
		t.Fatalf("remaining_likes = %d, want 0", home.RemainingLikes)
	}

	app.Clock.Advance(recoveryInterval)
	_, targets := app.NewStartedClients(t, 1)

	res := alice.MustLike(t, targets[0].ID)
	if res.LikesRecovered != 1 {
		t.Fatalf("likes_recovered = %d, want 1", res.LikesRecovered)
	}
	if res.RemainingLikes != 0 {
		t.Fatalf("remaining_likes = %d, want 0 after spending the recovered like", res.RemainingLikes)
	}
}

func TestDiscoverAlsoAppliesTheRecovery(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 2)
	app.Clock.Advance(recoveryInterval)

	// 探索画面は長く開かれたままになる。カードを継ぎ足すこの往復でも
	// 回復が反映される。
	res := alice.Discover(t)
	if res.LikesRecovered != 1 {
		t.Fatalf("likes_recovered = %d, want 1", res.LikesRecovered)
	}
	if want := matching.DailyLikeBudget - 2 + 1; res.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", res.RemainingLikes, want)
	}
	if res.LikeCapacity != matching.LikeCap {
		t.Fatalf("like_capacity = %d, want %d", res.LikeCapacity, matching.LikeCap)
	}
	requireNextRecovery(t, res.NextRecoveryAt, 18, 0)
}

func TestProfileRewardCombinesWithTimeRecovery(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 2)
	app.Clock.Advance(recoveryInterval)

	// 保存のトランザクションの中で、時間回復とプロフィール報酬が続けて入る。
	res := alice.CompleteProfile(t)
	if res.LikesRecovered != 1 {
		t.Fatalf("likes_recovered = %d, want 1", res.LikesRecovered)
	}
	if res.LikesGained != matching.ProfileCompletionReward {
		t.Fatalf("likes_gained = %d, want %d", res.LikesGained, matching.ProfileCompletionReward)
	}
	if want := matching.DailyLikeBudget - 2 + 1 + matching.ProfileCompletionReward; res.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", res.RemainingLikes, want)
	}
}

func TestMatchRewardCombinesWithTimeRecovery(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 2)
	app.Clock.Advance(recoveryInterval)

	// Like の1往復で、時間回復 +1・消費 -1・Match 報酬 +2 がまとめて起きる。
	res := mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID)
	if res.LikesRecovered != 1 {
		t.Fatalf("likes_recovered = %d, want 1", res.LikesRecovered)
	}
	if res.LikesGained != matching.MatchReward {
		t.Fatalf("likes_gained = %d, want %d", res.LikesGained, matching.MatchReward)
	}
	if want := matching.DailyLikeBudget - 2 + 1 - 1 + matching.MatchReward; res.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", res.RemainingLikes, want)
	}
}

// すべての回復手段を重ねても所持上限は破れない。DB の CHECK 制約もあるので、
// 破れていればここで書き込みが失敗する。
func TestEveryRecoverySourceTogetherStaysUnderTheCap(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)
	carol, carolCard := app.NewStartedClient(t)

	steps := []struct {
		name string
		run  func()
	}{
		{"profile", func() { alice.CompleteProfile(t) }},
		{"first match", func() { mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID) }},
		{"time recovery", func() { app.Clock.Advance(2 * recoveryInterval); alice.Home(t) }},
		{"second match", func() { mutualLike(t, alice, aliceCard.ID, carol, carolCard.ID) }},
	}
	for _, step := range steps {
		step.run()
		home := alice.Home(t)
		if home.RemainingLikes > matching.LikeCap {
			t.Fatalf("after %s remaining_likes = %d, over the cap %d", step.name, home.RemainingLikes, matching.LikeCap)
		}
		if state := app.RewardState(t, aliceCard.ID); state.LikeBalance > matching.LikeCap {
			t.Fatalf("after %s like_balance = %d, over the cap %d", step.name, state.LikeBalance, matching.LikeCap)
		}
	}
}

func TestTheRecoveryStateResetsAtTheDayBoundary(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 2)
	app.Clock.Advance(recoveryInterval)
	if home := alice.Home(t); home.LikesRecovered != 1 {
		t.Fatalf("likes_recovered = %d, want 1 on the first day", home.LikesRecovered)
	}
	if before := app.RewardState(t, aliceCard.ID); !before.RecoveryAnchorSet {
		t.Fatalf("reward state = %+v, want a running timer", before)
	}

	app.Clock.Advance(24 * time.Hour)

	// 新しいゲーム日は新しい persona で始まる。前日の起点も残らない。
	alice.Home(t)
	newAlice := alice.GeneratePersona(t)
	if newAlice.ID == aliceCard.ID {
		t.Fatal("expected a fresh persona for the new day")
	}

	fresh := apptest.RewardState{LikeBalance: matching.DailyLikeBudget}
	if state := app.RewardState(t, newAlice.ID); state != fresh {
		t.Fatalf("reward state = %+v, want a fresh %+v", state, fresh)
	}

	// 前日の起点が生きていれば、ここで回復が起きてしまう。
	app.Clock.Advance(recoveryInterval)
	home := alice.Home(t)
	if home.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0 — yesterday's timer must not carry over", home.LikesRecovered)
	}
	if home.RemainingLikes != matching.DailyLikeBudget {
		t.Fatalf("remaining_likes = %d, want the plain budget %d", home.RemainingLikes, matching.DailyLikeBudget)
	}
	if home.NextRecoveryAt != nil {
		t.Fatalf("next_recovery_at = %s, want null until the new day's first like", *home.NextRecoveryAt)
	}

	// 翌日のタイマーは、その日の初 Like から始まる。
	app.SpendLikes(t, alice, 1)
	app.Clock.Advance(recoveryInterval)
	if next := alice.Home(t); next.LikesRecovered != 1 {
		t.Fatalf("likes_recovered = %d, want 1 on the new day", next.LikesRecovered)
	}
}
