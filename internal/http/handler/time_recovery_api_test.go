package handler_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"kusamachi/internal/apptest"
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

func TestTheTimerStaysStoppedUntilTheFirstLikeIsSpent(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	// Like を使わずに何時間放置しても、初期の10に上積みはされない。
	app.Clock.Advance(3 * recoveryInterval)

	home := alice.Home(t)
	if home.RemainingLikes != matching.DailyLikeBudget {
		t.Fatalf("remaining_likes = %d, want the plain budget %d", home.RemainingLikes, matching.DailyLikeBudget)
	}
	if home.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0", home.LikesRecovered)
	}
	if home.NextRecoveryAt != nil {
		t.Fatalf("next_recovery_at = %s, want null before the first like", *home.NextRecoveryAt)
	}

	state := app.RewardState(t, aliceCard.ID)
	if state.RecoveryAnchorSet || state.TimeRecoveryCount != 0 {
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

	if state := app.RewardState(t, aliceCard.ID); state.TimeRecoveryCount != 1 {
		t.Fatalf("time_recovery_count = %d, want 1", state.TimeRecoveryCount)
	}
}

func TestTwoIntervalsRecoverTwoLikesAtOnce(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 3)
	app.Clock.Advance(2 * recoveryInterval)

	home := alice.Home(t)
	if want := matching.DailyLikeBudget - 3 + matching.MaxTimeRecoveries; home.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, want)
	}
	if home.LikesRecovered != matching.MaxTimeRecoveries {
		t.Fatalf("likes_recovered = %d, want %d", home.LikesRecovered, matching.MaxTimeRecoveries)
	}
	// 使い切ったのでタイマーは出さない。
	if home.NextRecoveryAt != nil {
		t.Fatalf("next_recovery_at = %s, want null at the daily limit", *home.NextRecoveryAt)
	}
	if state := app.RewardState(t, aliceCard.ID); state.TimeRecoveryCount != matching.MaxTimeRecoveries {
		t.Fatalf("time_recovery_count = %d, want %d", state.TimeRecoveryCount, matching.MaxTimeRecoveries)
	}
}

func TestTimeRecoveryStopsAtTwoPerDay(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	app.SpendLikes(t, alice, 5)

	// 3回ぶんの時間が経っていても、受け取れるのは2つまで。
	app.Clock.Advance(3 * recoveryInterval)
	if home := alice.Home(t); home.LikesRecovered != matching.MaxTimeRecoveries {
		t.Fatalf("likes_recovered = %d, want %d", home.LikesRecovered, matching.MaxTimeRecoveries)
	}

	// さらに待っても増えない。その日はもう時間では回復しない。
	app.Clock.Advance(2 * time.Hour)
	home := alice.Home(t)
	if home.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0 after the daily limit", home.LikesRecovered)
	}
	if want := matching.DailyLikeBudget - 5 + matching.MaxTimeRecoveries; home.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, want)
	}
	if state := app.RewardState(t, aliceCard.ID); state.TimeRecoveryCount != matching.MaxTimeRecoveries {
		t.Fatalf("time_recovery_count = %d, want %d", state.TimeRecoveryCount, matching.MaxTimeRecoveries)
	}
}

// 所持上限に達している間は回復せず、回復回数も消費しない。使い始めれば
// その経過時間がそのまま回復に変わる。
func TestTheCapBlocksRecoveryWithoutSpendingTheCount(t *testing.T) {
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

	app.Clock.Advance(2 * recoveryInterval)

	home := alice.Home(t)
	if home.LikesRecovered != 0 {
		t.Fatalf("likes_recovered = %d, want 0 at the cap", home.LikesRecovered)
	}
	if home.RemainingLikes != matching.LikeCap {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, matching.LikeCap)
	}
	// 満タンなので待つ意味が無く、タイマーも出さない。
	if home.NextRecoveryAt != nil {
		t.Fatalf("next_recovery_at = %s, want null at the cap", *home.NextRecoveryAt)
	}
	if state := app.RewardState(t, aliceCard.ID); state.TimeRecoveryCount != 0 {
		t.Fatalf("time_recovery_count = %d, want 0 — the count must survive the cap", state.TimeRecoveryCount)
	}

	// 2つ使えば、取り置かれていた2回ぶんがそのまま入る。Like の POST も
	// ホームと同じ lazy 評価を通るため、回復はその往復の中で先に反映される。
	// よって「どの応答が通知を運んだか」ではなく、最終状態で確かめる。
	app.SpendLikes(t, alice, 2)
	after := alice.Home(t)
	if after.RemainingLikes != matching.LikeCap {
		t.Fatalf("remaining_likes = %d, want back to %d", after.RemainingLikes, matching.LikeCap)
	}
	if state := app.RewardState(t, aliceCard.ID); state.TimeRecoveryCount != matching.MaxTimeRecoveries {
		t.Fatalf("time_recovery_count = %d, want %d once there was room", state.TimeRecoveryCount, matching.MaxTimeRecoveries)
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
	if state.TimeRecoveryCount != 1 {
		t.Fatalf("time_recovery_count = %d, want 1", state.TimeRecoveryCount)
	}
	if want := matching.DailyLikeBudget - 4 + 1; state.LikeBalance != want {
		t.Fatalf("like_balance = %d, want %d", state.LikeBalance, want)
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
	before := app.RewardState(t, aliceCard.ID)
	if !before.RecoveryAnchorSet || before.TimeRecoveryCount != 1 {
		t.Fatalf("reward state = %+v, want a running timer with one recovery", before)
	}

	app.Clock.Advance(24 * time.Hour)

	// 新しいゲーム日は新しい persona で始まる。前日の起点も回数も残らない。
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
