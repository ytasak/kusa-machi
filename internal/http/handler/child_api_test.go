package handler_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"kusamachi/internal/apperr"
	"kusamachi/internal/apptest"
	"kusamachi/internal/cleanup"
)

// matchedPair は Match が1つ成立した2人と、その match_id を返す。
func matchedPair(t *testing.T, app *apptest.App) (*apptest.Client, apptest.PersonaCard, *apptest.Client, apptest.PersonaCard, string) {
	t.Helper()
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)
	matchID := app.MatchBetween(t, alice, aliceCard, bob, bobCard)
	return alice, aliceCard, bob, bobCard, matchID
}

func TestMatchDetailShowsBothSidesAndNoChildYet(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard, _, bobCard, matchID := matchedPair(t, app)

	detail := alice.MatchDetail(t, matchID)

	if detail.MatchID != matchID {
		t.Fatalf("match_id = %s, want %s", detail.MatchID, matchID)
	}
	if detail.OwnPersona.ID != aliceCard.ID {
		t.Fatalf("own_persona = %s, want %s", detail.OwnPersona.ID, aliceCard.ID)
	}
	if detail.TargetPersona.ID != bobCard.ID {
		t.Fatalf("target_persona = %s, want %s", detail.TargetPersona.ID, bobCard.ID)
	}
	if detail.ChildGenerated || detail.Child != nil {
		t.Fatalf("a fresh match already reports a child: %+v", detail)
	}
}

func TestDrawingAChildStoresExactlyOne(t *testing.T) {
	app := apptest.New(t)
	alice, _, _, _, matchID := matchedPair(t, app)

	child := alice.MustDrawChild(t, matchID)

	if child.Gender == "" || child.Education == "" || child.Occupation == "" {
		t.Fatalf("child is missing attributes: %+v", child)
	}
	if child.HeightCm < 140 || child.HeightCm > 200 {
		t.Fatalf("child height = %d, out of 140-200", child.HeightCm)
	}
	if child.AnnualIncome%10 != 0 {
		t.Fatalf("child income = %d, not a multiple of 10", child.AnnualIncome)
	}
	if got := app.CountRows(t, "match_children"); got != 1 {
		t.Fatalf("match_children rows = %d, want 1", got)
	}

	detail := alice.MatchDetail(t, matchID)
	if !detail.ChildGenerated || detail.Child == nil {
		t.Fatalf("detail did not report the child: %+v", detail)
	}
	if *detail.Child != child {
		t.Fatalf("detail child = %+v, want %+v", *detail.Child, child)
	}
}

// TestDrawingAChildTwiceReturnsTheSameChild は引き直しができないことを確かめる。
// 2回目は再抽選ではなく、保存済みの子がそのまま返る。
func TestDrawingAChildTwiceReturnsTheSameChild(t *testing.T) {
	app := apptest.New(t)
	alice, _, _, _, matchID := matchedPair(t, app)

	first := alice.MustDrawChild(t, matchID)
	second := alice.MustDrawChild(t, matchID)

	if first != second {
		t.Fatalf("the second draw rerolled the child: %+v then %+v", first, second)
	}
	if got := app.CountRows(t, "match_children"); got != 1 {
		t.Fatalf("match_children rows = %d, want 1", got)
	}
}

// TestBothSidesSeeTheSameChild は、Match は二人の出来事なので、どちらから
// 引いても同じ子になることを確かめる。相手が先に引いていれば、その子を見る。
func TestBothSidesSeeTheSameChild(t *testing.T) {
	app := apptest.New(t)
	alice, _, bob, _, matchID := matchedPair(t, app)

	fromAlice := alice.MustDrawChild(t, matchID)
	fromBob := bob.MustDrawChild(t, matchID)

	if fromAlice != fromBob {
		t.Fatalf("the two sides see different children: %+v vs %+v", fromAlice, fromBob)
	}
	if got := app.CountRows(t, "match_children"); got != 1 {
		t.Fatalf("match_children rows = %d, want 1", got)
	}
}

// TestConcurrentDrawsCreateOneChild は多重クリックと再送の再現。
// UNIQUE(match_id) が効いていれば、何本同時に来ても子は1人しか生まれない。
func TestConcurrentDrawsCreateOneChild(t *testing.T) {
	app := apptest.New(t)
	alice, _, bob, _, matchID := matchedPair(t, app)

	const attempts = 8
	children := make([]apptest.ChildCard, attempts)

	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 両者から同時に押される場合も混ぜる。
			client := alice
			if i%2 == 1 {
				client = bob
			}
			res := client.DrawChild(t, matchID)
			if res.Status != http.StatusOK {
				return
			}
			res.Decode(t, &children[i])
		}()
	}
	wg.Wait()

	if got := app.CountRows(t, "match_children"); got != 1 {
		t.Fatalf("match_children rows = %d, want 1", got)
	}
	for i, c := range children {
		if c == (apptest.ChildCard{}) {
			t.Fatalf("concurrent draw %d did not return a child", i)
		}
		if c != children[0] {
			t.Fatalf("concurrent draw %d returned a different child: %+v vs %+v", i, c, children[0])
		}
	}
}

func TestOtherPeoplesMatchCannotBeOpenedOrDrawn(t *testing.T) {
	app := apptest.New(t)
	_, _, _, _, matchID := matchedPair(t, app)

	outsider, _ := app.NewStartedClient(t)

	outsider.Do(t, http.MethodGet, "/api/matches/"+matchID, nil).
		RequireError(t, apperr.CodeMatchUnavailable)
	outsider.DrawChild(t, matchID).RequireError(t, apperr.CodeMatchUnavailable)

	if got := app.CountRows(t, "match_children"); got != 0 {
		t.Fatalf("match_children rows = %d, want 0", got)
	}
}

func TestUnknownMatchIsRejected(t *testing.T) {
	app := apptest.New(t)
	alice, _ := app.NewStartedClient(t)

	unknown := uuid.NewString()
	alice.Do(t, http.MethodGet, "/api/matches/"+unknown, nil).
		RequireError(t, apperr.CodeMatchUnavailable)
	alice.DrawChild(t, unknown).RequireError(t, apperr.CodeMatchUnavailable)

	alice.Do(t, http.MethodGet, "/api/matches/not-a-uuid", nil).
		RequireError(t, apperr.CodeInvalidRequest)
	alice.DrawChild(t, "not-a-uuid").RequireError(t, apperr.CodeInvalidRequest)
}

func TestDrawingAChildRequiresCSRF(t *testing.T) {
	app := apptest.New(t)
	alice, _, _, _, matchID := matchedPair(t, app)

	alice.DoWithCSRF(t, http.MethodPost, "/api/matches/"+matchID+"/child", nil, "forged").
		RequireError(t, apperr.CodeInvalidCSRF)

	if got := app.CountRows(t, "match_children"); got != 0 {
		t.Fatalf("match_children rows = %d, want 0", got)
	}
}

func TestMatchListFlagsTheChild(t *testing.T) {
	app := apptest.New(t)
	alice, _, _, bobCard, matchID := matchedPair(t, app)

	list := alice.Matches(t)
	if len(list.Personas) != 1 {
		t.Fatalf("matches = %d, want 1", len(list.Personas))
	}
	if list.Personas[0].ID != bobCard.ID {
		t.Fatalf("match list returned %s, want the counterpart %s", list.Personas[0].ID, bobCard.ID)
	}
	if list.Personas[0].MatchID != matchID {
		t.Fatalf("match list match_id = %s, want %s", list.Personas[0].MatchID, matchID)
	}
	if list.Personas[0].ChildGenerated {
		t.Fatal("match list reports a child before one was drawn")
	}

	alice.MustDrawChild(t, matchID)

	if got := alice.Matches(t); !got.Personas[0].ChildGenerated {
		t.Fatal("match list did not pick up the drawn child")
	}
}

// TestYesterdaysChildIsGoneToday は、子も当日限定であることを確かめる。
// 日をまたいだ時点で Match ごと引けなくなり、翌日に持ち越されない。
func TestYesterdaysChildIsGoneToday(t *testing.T) {
	app := apptest.New(t)
	alice, _, _, _, matchID := matchedPair(t, app)
	alice.MustDrawChild(t, matchID)

	app.Clock.Advance(24 * time.Hour)
	alice.Start(t)

	alice.Do(t, http.MethodGet, "/api/matches/"+matchID, nil).
		RequireError(t, apperr.CodeMatchUnavailable)
	alice.DrawChild(t, matchID).RequireError(t, apperr.CodeMatchUnavailable)

	if list := alice.Matches(t); len(list.Personas) != 0 {
		t.Fatalf("yesterday's match is still listed: %+v", list.Personas)
	}
}

// TestCleanupDeletesChildrenWithTheirMatch は、日次の物理削除が子まで
// CASCADE で届くことを確かめる。
func TestCleanupDeletesChildrenWithTheirMatch(t *testing.T) {
	app := apptest.New(t)
	alice, _, _, _, matchID := matchedPair(t, app)
	alice.MustDrawChild(t, matchID)

	if got := app.CountRows(t, "match_children"); got != 1 {
		t.Fatalf("match_children rows = %d, want 1", got)
	}

	app.Clock.Advance(24 * time.Hour)

	job := cleanup.NewJob(app.Pool, app.Clock, app.Photos)
	if _, err := job.RunOnce(t.Context()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	for _, table := range []string{"matches", "match_children"} {
		if got := app.CountRows(t, table); got != 0 {
			t.Fatalf("%s still has %d rows after cleanup", table, got)
		}
	}
}

func TestMatchDetailRequiresAGeneratedPersona(t *testing.T) {
	app := apptest.New(t)
	_, _, _, _, matchID := matchedPair(t, app)

	newcomer := app.NewClient()
	newcomer.Home(t)

	newcomer.Do(t, http.MethodGet, "/api/matches/"+matchID, nil).
		RequireError(t, apperr.CodePersonaNotGenerated)
	newcomer.DrawChild(t, matchID).RequireError(t, apperr.CodePersonaNotGenerated)
}
