package handler_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"kusamachi/internal/apperr"
	"kusamachi/internal/apptest"
	"kusamachi/internal/matching"
)

func TestLikeSpendsBudget(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)
	_, bobCard := app.NewStartedClient(t)

	res := alice.MustLike(t, bobCard.ID)

	if res.RemainingLikes != matching.DailyLikeBudget-1 {
		t.Fatalf("remaining_likes = %d, want %d", res.RemainingLikes, matching.DailyLikeBudget-1)
	}
	if res.Matched {
		t.Fatal("a one-sided like must not create a match")
	}
	if res.MatchID != nil || res.TargetPersona != nil {
		t.Fatalf("unmatched like leaked match fields: %+v", res)
	}
}

func TestMutualLikeCreatesExactlyOneMatch(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)

	alice.MustLike(t, bobCard.ID)
	res := bob.MustLike(t, aliceCard.ID)

	if !res.Matched {
		t.Fatal("mutual like did not report a match")
	}
	if res.MatchID == nil {
		t.Fatal("match_id is missing")
	}
	if res.TargetPersona == nil || res.TargetPersona.ID != aliceCard.ID {
		t.Fatalf("target_persona = %+v, want alice", res.TargetPersona)
	}
	if got := app.CountRows(t, "matches"); got != 1 {
		t.Fatalf("match rows = %d, want 1", got)
	}

	home := bob.Home(t)
	if home.MatchCount != 1 || !home.HasUnseenMatches {
		t.Fatalf("home did not reflect the match: %+v", home)
	}
}

func TestSelfLikeAndSelfPassAreRejected(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)

	alice.Like(t, aliceCard.ID).RequireError(t, apperr.CodeSelfActionNotAllowed)
	alice.Pass(t, aliceCard.ID).RequireError(t, apperr.CodeSelfActionNotAllowed)
}

func TestDuplicateLikeDoesNotConsumeBudgetTwice(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)
	_, bobCard := app.NewStartedClient(t)

	alice.MustLike(t, bobCard.ID)
	alice.Like(t, bobCard.ID).RequireError(t, apperr.CodeAlreadyLiked)

	if got := app.CountRows(t, "likes"); got != 1 {
		t.Fatalf("like rows = %d, want 1", got)
	}
	if home := alice.Home(t); home.RemainingLikes != matching.DailyLikeBudget-1 {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, matching.DailyLikeBudget-1)
	}
}

func TestTenLikesSucceedAndTheEleventhFails(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)
	_, targets := app.NewStartedClients(t, matching.DailyLikeBudget+1)

	for i := range matching.DailyLikeBudget {
		res := alice.MustLike(t, targets[i].ID)
		want := matching.DailyLikeBudget - 1 - i
		if res.RemainingLikes != want {
			t.Fatalf("like %d: remaining_likes = %d, want %d", i+1, res.RemainingLikes, want)
		}
	}

	alice.Like(t, targets[matching.DailyLikeBudget].ID).RequireError(t, apperr.CodeLikeLimitExceeded)

	if got := app.CountRows(t, "likes"); got != matching.DailyLikeBudget {
		t.Fatalf("like rows = %d, want %d", got, matching.DailyLikeBudget)
	}
}

func TestConcurrentLikesNeverExceedTheBudget(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)

	const targetCount = 20
	_, targets := app.NewStartedClients(t, targetCount)

	var (
		mu        sync.Mutex
		succeeded int
		rejected  int
		wg        sync.WaitGroup
	)

	for _, target := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := alice.Like(t, target.ID)

			mu.Lock()
			defer mu.Unlock()
			switch res.Status {
			case http.StatusOK:
				succeeded++
			case http.StatusUnprocessableEntity:
				rejected++
			default:
				t.Errorf("unexpected status %d: %s", res.Status, res.Body)
			}
		}()
	}
	wg.Wait()

	if succeeded != matching.DailyLikeBudget {
		t.Fatalf("succeeded = %d, want %d", succeeded, matching.DailyLikeBudget)
	}
	if rejected != targetCount-matching.DailyLikeBudget {
		t.Fatalf("rejected = %d, want %d", rejected, targetCount-matching.DailyLikeBudget)
	}
	if got := app.CountRows(t, "likes"); got != matching.DailyLikeBudget {
		t.Fatalf("like rows = %d, want %d", got, matching.DailyLikeBudget)
	}
}

func TestConcurrentMutualLikesCreateOneMatch(t *testing.T) {
	// 双方が同じ瞬間に相互 Like する。どちらも相手の Like を見落としてはならず、
	// Match の行が2件できてもいけない。
	for range 5 {
		app := apptest.New(t)
		alice, aliceCard := app.NewStartedClient(t)
		bob, bobCard := app.NewStartedClient(t)

		var (
			wg      sync.WaitGroup
			matched [2]bool
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			matched[0] = alice.MustLike(t, bobCard.ID).Matched
		}()
		go func() {
			defer wg.Done()
			matched[1] = bob.MustLike(t, aliceCard.ID).Matched
		}()
		wg.Wait()

		if got := app.CountRows(t, "matches"); got != 1 {
			t.Fatalf("match rows = %d, want 1", got)
		}
		if !matched[0] && !matched[1] {
			t.Fatal("neither side observed the match")
		}
	}
}

func TestPassCountsUpWithoutExcluding(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)
	_, bobCard := app.NewStartedClient(t)

	// 上限は無いので、かつて打ち止めだった3回を超えても受け付ける。
	const passes = 5
	for i := 1; i <= passes; i++ {
		res := alice.MustPass(t, bobCard.ID)
		if res.PassCount != i {
			t.Fatalf("pass %d: pass_count = %d", i, res.PassCount)
		}
	}

	var stored int
	err := app.Pool.QueryRow(t.Context(),
		`SELECT pass_count FROM passes WHERE to_persona_id = $1`, bobCard.ID).Scan(&stored)
	if err != nil {
		t.Fatalf("read pass_count: %v", err)
	}
	if stored != passes {
		t.Fatalf("stored pass_count = %d, want %d", stored, passes)
	}

	// 何度 Pass しても候補から消えない。除外はフロントエンドのクールダウンだけ。
	if !idSet(alice.Discover(t).Personas)[bobCard.ID] {
		t.Error("discover dropped a persona that was passed repeatedly")
	}
}

func TestPassIsRejectedAfterLike(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)
	_, bobCard := app.NewStartedClient(t)

	alice.MustLike(t, bobCard.ID)
	alice.Pass(t, bobCard.ID).RequireError(t, apperr.CodeAlreadyLiked)
}

func TestExposureCountsEvaluationsOnly(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)
	_, bobCard := app.NewStartedClient(t)

	// Discover のバッチ取得で bob は表示されるが、評価としては数えない。
	alice.Discover(t)
	alice.Discover(t)
	if got := app.ExposureCount(t, bobCard.ID); got != 0 {
		t.Fatalf("exposure after discover = %d, want 0", got)
	}

	alice.MustPass(t, bobCard.ID)
	if got := app.ExposureCount(t, bobCard.ID); got != 1 {
		t.Fatalf("exposure after pass = %d, want 1", got)
	}

	alice.MustLike(t, bobCard.ID)
	if got := app.ExposureCount(t, bobCard.ID); got != 2 {
		t.Fatalf("exposure after like = %d, want 2", got)
	}

	// 拒否された操作は数えない。
	alice.Like(t, bobCard.ID).RequireError(t, apperr.CodeAlreadyLiked)
	if got := app.ExposureCount(t, bobCard.ID); got != 2 {
		t.Fatalf("exposure after rejected like = %d, want 2", got)
	}
}

func TestDiscoverExcludesEverythingItShould(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	_, targets := app.NewStartedClients(t, 3)

	liked, cooldown, plain := targets[0], targets[1], targets[2]

	alice.MustLike(t, liked.ID)

	got := idSet(alice.Discover(t, cooldown.ID).Personas)

	if got[aliceCard.ID] {
		t.Error("discover returned the requester's own persona")
	}
	if got[liked.ID] {
		t.Error("discover returned an already liked persona")
	}
	if got[cooldown.ID] {
		t.Error("discover ignored the frontend cooldown exclusion")
	}
	if !got[plain.ID] {
		t.Error("discover did not return an eligible persona")
	}
}

func TestDiscoverReturnsAtMostFiveDistinctPersonas(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)
	app.NewStartedClients(t, 8)

	batch := alice.Discover(t).Personas
	if len(batch) != 5 {
		t.Fatalf("batch size = %d, want 5", len(batch))
	}
	if len(idSet(batch)) != len(batch) {
		t.Fatal("batch contains duplicate personas")
	}
}

func TestDiscoverPrefersLeastExposedPersonas(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)
	_, targets := app.NewStartedClients(t, 6)

	// 5人に exposure を与える。6人目は手つかずなので、alice が受け取る
	// どのバッチにも必ず含まれるはず。
	rater, _ := app.NewStartedClient(t)
	for _, target := range targets[:5] {
		rater.MustPass(t, target.ID)
	}

	fresh := targets[5]
	for range 5 {
		if !idSet(alice.Discover(t).Personas)[fresh.ID] {
			t.Fatal("least exposed persona was not prioritised")
		}
	}
}

func TestDiscoverOnlyReturnsTodaysPersonas(t *testing.T) {
	app := apptest.New(t)
	_, yesterday := app.NewStartedClient(t)

	app.Clock.Advance(24 * time.Hour)

	alice, _ := app.NewStartedClient(t)
	_, today := app.NewStartedClient(t)

	got := idSet(alice.Discover(t).Personas)
	if got[yesterday.ID] {
		t.Error("yesterday's persona appeared in today's market")
	}
	if !got[today.ID] {
		t.Error("today's persona is missing from the market")
	}
}

func TestYesterdaysPersonaCannotBeLikedToday(t *testing.T) {
	app := apptest.New(t)
	_, yesterday := app.NewStartedClient(t)

	app.Clock.Advance(24 * time.Hour)

	alice, _ := app.NewStartedClient(t)
	alice.Like(t, yesterday.ID).RequireError(t, apperr.CodeTargetPersonaUnavailable)
	alice.Pass(t, yesterday.ID).RequireError(t, apperr.CodeTargetPersonaUnavailable)

	if got := app.CountRows(t, "likes"); got != 0 {
		t.Fatalf("like rows = %d, want 0", got)
	}
}

func TestOldLikesAndMatchesDoNotLeakIntoTheNewDay(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)

	alice.MustLike(t, bobCard.ID)
	bob.MustLike(t, aliceCard.ID)

	app.Clock.Advance(24 * time.Hour)

	alice.Home(t)
	alice.GeneratePersona(t)

	home := alice.Home(t)
	if home.ReceivedLikeCount != 0 || home.MatchCount != 0 {
		t.Fatalf("yesterday's counters leaked: %+v", home)
	}
	if home.RemainingLikes != matching.DailyLikeBudget {
		t.Fatalf("like budget did not reset: %d", home.RemainingLikes)
	}
	if home.HasUnseenLikes || home.HasUnseenMatches {
		t.Fatalf("stale unseen badges: %+v", home)
	}
}

func TestUnknownTargetIsRejected(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)

	unknown := uuid.NewString()
	alice.Like(t, unknown).RequireError(t, apperr.CodeTargetPersonaUnavailable)
	alice.Pass(t, unknown).RequireError(t, apperr.CodeTargetPersonaUnavailable)

	alice.Do(t, http.MethodPost, "/api/likes", map[string]any{"persona_id": "not-a-uuid"}).
		RequireError(t, apperr.CodeInvalidRequest)
}

func TestActionsRequireAGeneratedPersona(t *testing.T) {
	app := apptest.New(t)
	_, bobCard := app.NewStartedClient(t)

	newcomer := app.NewClient()
	newcomer.Home(t)

	newcomer.Like(t, bobCard.ID).RequireError(t, apperr.CodePersonaNotGenerated)
	newcomer.Pass(t, bobCard.ID).RequireError(t, apperr.CodePersonaNotGenerated)
	newcomer.Do(t, http.MethodGet, "/api/discover", nil).RequireError(t, apperr.CodePersonaNotGenerated)
}

func idSet(cards []apptest.PersonaCard) map[string]bool {
	out := make(map[string]bool, len(cards))
	for _, c := range cards {
		out[c.ID] = true
	}
	return out
}
