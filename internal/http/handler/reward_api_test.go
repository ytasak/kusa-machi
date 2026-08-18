package handler_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"kusamachi/internal/apperr"
	"kusamachi/internal/apptest"
	"kusamachi/internal/matching"
)

// mutualLike は先に相手から Like させ、実行者の Like で Match を成立させる。
// 戻り値は Match を成立させた側の応答。
func mutualLike(t *testing.T, actor *apptest.Client, actorID string, counterpart *apptest.Client, counterpartID string) apptest.LikeResponse {
	t.Helper()
	counterpart.MustLike(t, actorID)
	res := actor.MustLike(t, counterpartID)
	if !res.Matched {
		t.Fatalf("mutual like did not report a match: %+v", res)
	}
	return res
}

func TestProfileCompletionGrantsOneLike(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	res := alice.CompleteProfile(t)

	if res.LikesGained != matching.ProfileCompletionReward {
		t.Fatalf("likes_gained = %d, want %d", res.LikesGained, matching.ProfileCompletionReward)
	}
	if want := matching.DailyLikeBudget + 1; res.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", res.RemainingLikes, want)
	}

	// ホームを開き直しても回復は残っている。残数はサーバが導出するので、
	// 報酬が状態として保存されていなければここで元に戻ってしまう。
	if home := alice.Home(t); home.RemainingLikes != matching.DailyLikeBudget+1 {
		t.Fatalf("home remaining_likes = %d, want %d", home.RemainingLikes, matching.DailyLikeBudget+1)
	}

	state := app.RewardState(t, aliceCard.ID)
	if state.BonusLikes != 1 || !state.ProfileRewardClaimed {
		t.Fatalf("reward state = %+v, want bonus 1 and claimed", state)
	}
}

func TestPartialProfileGrantsNothing(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	// 3項目のうち1つでも欠けていれば報酬は出ない。1つずつ埋めていって、
	// 最後の1つが入るまで一度も回復しないことを確かめる。
	partials := []map[string]any{
		{"name": "さとし"},
		{"name": "さとし", "hobby": "散歩"},
		{"hobby": "散歩", "bio": "よろしく"},
		{"name": "さとし", "bio": "よろしく"},
	}
	for _, fields := range partials {
		res := alice.MustUpdateProfile(t, fields)
		if res.LikesGained != 0 {
			t.Fatalf("partial profile %v granted %d likes", fields, res.LikesGained)
		}
		if res.RemainingLikes != matching.DailyLikeBudget {
			t.Fatalf("partial profile %v changed remaining to %d", fields, res.RemainingLikes)
		}
	}

	if state := app.RewardState(t, aliceCard.ID); state.BonusLikes != 0 || state.ProfileRewardClaimed {
		t.Fatalf("reward state = %+v, want untouched", state)
	}

	// 3つ揃った時点で初めて出る。
	if res := alice.CompleteProfile(t); res.LikesGained != 1 {
		t.Fatalf("completed profile granted %d likes, want 1", res.LikesGained)
	}
}

func TestProfileRewardIsGrantedOnlyOncePerDay(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	if res := alice.CompleteProfile(t); res.LikesGained != 1 {
		t.Fatalf("first completion granted %d, want 1", res.LikesGained)
	}

	// 同じ PATCH の再送。ページ再読み込み後の保存もこれと同じ形になる。
	second := alice.CompleteProfile(t)
	if second.LikesGained != 0 {
		t.Fatalf("resending the same profile granted %d likes", second.LikesGained)
	}
	if want := matching.DailyLikeBudget + 1; second.RemainingLikes != want {
		t.Fatalf("remaining_likes = %d, want %d", second.RemainingLikes, want)
	}

	if state := app.RewardState(t, aliceCard.ID); state.BonusLikes != 1 {
		t.Fatalf("bonus_likes = %d, want 1 after a resend", state.BonusLikes)
	}
}

func TestProfileRewardDoesNotReturnAfterClearingFields(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	alice.CompleteProfile(t)

	// 一度受け取った後に項目を消して入れ直す。受け取り済みのフラグは
	// 下がらないので、もう一度もらえることはない。
	alice.MustUpdateProfile(t, map[string]any{"name": "", "hobby": "", "bio": ""})
	again := alice.CompleteProfile(t)

	if again.LikesGained != 0 {
		t.Fatalf("re-completing the profile granted %d likes", again.LikesGained)
	}
	if state := app.RewardState(t, aliceCard.ID); state.BonusLikes != 1 {
		t.Fatalf("bonus_likes = %d, want 1", state.BonusLikes)
	}
}

func TestProfileRewardIsClippedAtTheLikeCap(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)
	carol, carolCard := app.NewStartedClient(t)

	// Match 2回で所持上限まで押し上げる。
	mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID)
	last := mutualLike(t, alice, aliceCard.ID, carol, carolCard.ID)
	if last.RemainingLikes != matching.LikeCap {
		t.Fatalf("remaining_likes = %d, want the cap %d", last.RemainingLikes, matching.LikeCap)
	}

	// 上限に達している状態でプロフィールを完成させる。回復量は 0 だが、
	// 受け取り枠は消費される。溢れた分は仕様どおり失われる。
	res := alice.CompleteProfile(t)
	if res.LikesGained != 0 {
		t.Fatalf("likes_gained = %d, want 0 at the cap", res.LikesGained)
	}
	if res.RemainingLikes != matching.LikeCap {
		t.Fatalf("remaining_likes = %d, want %d", res.RemainingLikes, matching.LikeCap)
	}

	state := app.RewardState(t, aliceCard.ID)
	if !state.ProfileRewardClaimed {
		t.Fatal("the profile reward slot must be consumed even when nothing was gained")
	}
	if state.BonusLikes != matching.MatchReward*2 {
		t.Fatalf("bonus_likes = %d, want %d", state.BonusLikes, matching.MatchReward*2)
	}
}

func TestMatchRewardIsGrantedTwiceThenStops(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	partners, partnerCards := app.NewStartedClients(t, 3)

	// 1回目と2回目は満額、3回目は打ち止めで回復なし。
	for i, want := range []int{matching.MatchReward, matching.MatchReward, 0} {
		res := mutualLike(t, alice, aliceCard.ID, partners[i], partnerCards[i].ID)
		if res.LikesGained != want {
			t.Fatalf("match %d: likes_gained = %d, want %d", i+1, res.LikesGained, want)
		}
	}

	state := app.RewardState(t, aliceCard.ID)
	if state.MatchRewardCount != matching.MaxMatchRewards {
		t.Fatalf("match_reward_count = %d, want %d", state.MatchRewardCount, matching.MaxMatchRewards)
	}
	if want := matching.MatchReward * matching.MaxMatchRewards; state.BonusLikes != want {
		t.Fatalf("bonus_likes = %d, want %d", state.BonusLikes, want)
	}
}

func TestMatchRewardGoesToBothSides(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)

	mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID)

	// 先に Like して待っていた側も同じ報酬を受け取る。bob の画面はこの後
	// ホームを読んだときに新しい残数を得る。
	for _, c := range []struct {
		name string
		id   string
		home apptest.HomeResponse
	}{
		{"alice", aliceCard.ID, alice.Home(t)},
		{"bob", bobCard.ID, bob.Home(t)},
	} {
		state := app.RewardState(t, c.id)
		if state.MatchRewardCount != 1 || state.BonusLikes != matching.MatchReward {
			t.Fatalf("%s reward state = %+v, want one match reward", c.name, state)
		}
		// Like を1つ使って Match したので、残数は 10 - 1 + 2。
		if want := matching.DailyLikeBudget - 1 + matching.MatchReward; c.home.RemainingLikes != want {
			t.Fatalf("%s remaining_likes = %d, want %d", c.name, c.home.RemainingLikes, want)
		}
	}
}

func TestResendingALikedMatchGrantsNothingExtra(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)

	mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID)
	before := app.RewardState(t, aliceCard.ID)

	// 同じ Like の再送。どちらの向きから来ても Match は増えず、報酬も動かない。
	alice.Like(t, bobCard.ID).RequireError(t, apperr.CodeAlreadyLiked)
	bob.Like(t, aliceCard.ID).RequireError(t, apperr.CodeAlreadyLiked)

	if got := app.CountRows(t, "matches"); got != 1 {
		t.Fatalf("match rows = %d, want 1", got)
	}
	for _, c := range []struct {
		name string
		id   string
	}{{"alice", aliceCard.ID}, {"bob", bobCard.ID}} {
		if got := app.RewardState(t, c.id); got != before {
			t.Fatalf("%s reward state changed on resend: %+v, want %+v", c.name, got, before)
		}
	}
}

func TestConcurrentMatchesNeverExceedTheRewardLimit(t *testing.T) {
	// 5人が先に alice へ Like を送った状態で、alice が全員へ同時に Like する。
	// Match は5件成立するが、報酬は2回で打ち止めでなければならない。
	const partnerCount = 5

	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	partners, partnerCards := app.NewStartedClients(t, partnerCount)

	for _, p := range partners {
		p.MustLike(t, aliceCard.ID)
	}

	var wg sync.WaitGroup
	for _, card := range partnerCards {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res := alice.Like(t, card.ID); res.Status != http.StatusOK {
				t.Errorf("unexpected status %d: %s", res.Status, res.Body)
			}
		}()
	}
	wg.Wait()

	if got := app.CountRows(t, "matches"); got != partnerCount {
		t.Fatalf("match rows = %d, want %d", got, partnerCount)
	}

	state := app.RewardState(t, aliceCard.ID)
	if state.MatchRewardCount != matching.MaxMatchRewards {
		t.Fatalf("match_reward_count = %d, want exactly %d", state.MatchRewardCount, matching.MaxMatchRewards)
	}
	if want := matching.MatchReward * matching.MaxMatchRewards; state.BonusLikes != want {
		t.Fatalf("bonus_likes = %d, want %d", state.BonusLikes, want)
	}
	if home := alice.Home(t); home.RemainingLikes > matching.LikeCap {
		t.Fatalf("remaining_likes = %d, over the cap %d", home.RemainingLikes, matching.LikeCap)
	}
}

func TestMatchRewardIsClippedAtTheLikeCap(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)
	carol, carolCard := app.NewStartedClient(t)

	// 1回目の Match で残り 11 になり、プロフィール完成で上限の 12 に届く。
	if res := mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID); res.RemainingLikes != 11 {
		t.Fatalf("after the first match remaining = %d, want 11", res.RemainingLikes)
	}
	if res := alice.CompleteProfile(t); res.RemainingLikes != matching.LikeCap {
		t.Fatalf("after completing the profile remaining = %d, want %d", res.RemainingLikes, matching.LikeCap)
	}

	// 2回目の Match。Like を1つ使って残り 11 になった状態からの回復なので、
	// 満額の +2 ではなく上限までの +1 しか入らない。
	res := mutualLike(t, alice, aliceCard.ID, carol, carolCard.ID)
	if res.LikesGained != 1 {
		t.Fatalf("likes_gained = %d, want 1 clipped by the cap", res.LikesGained)
	}
	if res.RemainingLikes != matching.LikeCap {
		t.Fatalf("remaining_likes = %d, want %d", res.RemainingLikes, matching.LikeCap)
	}
}

func TestRecoveredLikesAreActuallySpendable(t *testing.T) {
	// 仕様の「その日に使えるのは最大15回程度」を実際に数える。
	// 予算 10 に、プロフィール完成 +1 と Match 2回ぶんの +4 が乗る。
	const maxUsable = matching.DailyLikeBudget +
		matching.ProfileCompletionReward +
		matching.MatchReward*matching.MaxMatchRewards

	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)
	carol, carolCard := app.NewStartedClient(t)

	sent := 0
	alice.CompleteProfile(t)

	// 満額の回復を受けるには、所持上限に余裕がある状態で Match する必要がある。
	// つまり「使いながら回復する」しかない。まず使ってから Match を成立させる。
	spend := func(n int) {
		_, targets := app.NewStartedClients(t, n)
		for _, target := range targets {
			alice.MustLike(t, target.ID)
			sent++
		}
	}

	spend(2)
	mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID)
	sent++

	spend(1)
	mutualLike(t, alice, aliceCard.ID, carol, carolCard.ID)
	sent++

	// ここで回復は満額入っている。DB の CHECK 制約の上限とも一致する。
	if state := app.RewardState(t, aliceCard.ID); state.BonusLikes != maxUsable-matching.DailyLikeBudget {
		t.Fatalf("bonus_likes = %d, want %d", state.BonusLikes, maxUsable-matching.DailyLikeBudget)
	}

	// 残りを使い切る。回復ぶんがちゃんと使えるかを、上限で弾かれるまで
	// 数えることで確かめる。
	_, spares := app.NewStartedClients(t, maxUsable-sent+1)
	for i, target := range spares {
		res := alice.Like(t, target.ID)
		if i == len(spares)-1 {
			res.RequireError(t, apperr.CodeLikeLimitExceeded)
			break
		}
		res.RequireStatus(t, http.StatusOK)
		sent++
	}

	if sent != maxUsable {
		t.Fatalf("spendable likes = %d, want %d", sent, maxUsable)
	}
	if home := alice.Home(t); home.RemainingLikes != 0 {
		t.Fatalf("remaining_likes = %d, want 0", home.RemainingLikes)
	}
}

func TestRewardStateResetsAtTheDayBoundary(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)

	alice.CompleteProfile(t)
	mutualLike(t, alice, aliceCard.ID, bob, bobCard.ID)

	before := app.RewardState(t, aliceCard.ID)
	if !before.ProfileRewardClaimed || before.MatchRewardCount != 1 {
		t.Fatalf("reward state = %+v, want a claimed profile and one match", before)
	}

	app.Clock.Advance(24 * time.Hour)

	// 新しいゲーム日は新しい persona で始まる。前日の報酬状態は
	// 引き継がれない。
	alice.Home(t)
	newAlice := alice.GeneratePersona(t)
	bob.Home(t)
	newBob := bob.GeneratePersona(t)

	if newAlice.ID == aliceCard.ID {
		t.Fatal("expected a fresh persona for the new day")
	}
	if state := app.RewardState(t, newAlice.ID); state != (apptest.RewardState{}) {
		t.Fatalf("reward state = %+v, want everything back to zero", state)
	}
	if home := alice.Home(t); home.RemainingLikes != matching.DailyLikeBudget {
		t.Fatalf("remaining_likes = %d, want the plain budget %d", home.RemainingLikes, matching.DailyLikeBudget)
	}

	// プロフィール報酬はもう一度受け取れる。
	if res := alice.CompleteProfile(t); res.LikesGained != matching.ProfileCompletionReward {
		t.Fatalf("new day profile reward = %d, want %d", res.LikesGained, matching.ProfileCompletionReward)
	}

	// Match 報酬の回数も 0 に戻っているので、満額から数え直す。
	if res := mutualLike(t, alice, newAlice.ID, bob, newBob.ID); res.LikesGained != matching.MatchReward {
		t.Fatalf("new day match reward = %d, want %d", res.LikesGained, matching.MatchReward)
	}
	if state := app.RewardState(t, newAlice.ID); state.MatchRewardCount != 1 {
		t.Fatalf("match_reward_count = %d, want 1", state.MatchRewardCount)
	}
}

func TestProfileRewardAvailabilityIsExposedToTheClient(t *testing.T) {
	// 画面が事前の訴求を出せるかは、この1つのフラグだけで決まる。
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)

	if home := alice.Home(t); !home.ProfileRewardAvailable {
		t.Fatal("a fresh persona must be able to earn the profile reward")
	}

	// 途中まで埋めただけなら、まだ受け取れる。
	if res := alice.MustUpdateProfile(t, map[string]any{"name": "さとし"}); !res.ProfileRewardAvailable {
		t.Fatal("a partial profile must keep the reward available")
	}

	// 完成させた保存の応答で、その場で false に変わる。画面はここで訴求を引っ込める。
	if res := alice.CompleteProfile(t); res.ProfileRewardAvailable {
		t.Fatal("the reward must not look available right after it was claimed")
	}
	if home := alice.Home(t); home.ProfileRewardAvailable {
		t.Fatal("home must not offer a reward that was already claimed")
	}

	// 項目を消しても復活しない。取れない報酬を誘導しないため。
	alice.MustUpdateProfile(t, map[string]any{"name": "", "hobby": "", "bio": ""})
	if home := alice.Home(t); home.ProfileRewardAvailable {
		t.Fatal("clearing the fields must not make the reward look available again")
	}

	// 翌日は新しい persona なので、また訴求できる。
	app.Clock.Advance(24 * time.Hour)
	alice.Home(t)
	alice.GeneratePersona(t)
	if home := alice.Home(t); !home.ProfileRewardAvailable {
		t.Fatal("a new game day must offer the profile reward again")
	}
}
