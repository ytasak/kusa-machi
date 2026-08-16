package handler_test

import (
	"net/http"
	"testing"
	"time"

	"kusamachi/internal/apperr"
	"kusamachi/internal/apptest"
	"kusamachi/internal/cleanup"
	"kusamachi/internal/clock"
)

func TestReceivedLikesListsSendersNewestFirst(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	senders, senderCards := app.NewStartedClients(t, 3)

	for _, sender := range senders {
		sender.MustLike(t, aliceCard.ID)
	}

	res := alice.Do(t, http.MethodGet, "/api/likes/received", nil)
	res.RequireStatus(t, http.StatusOK)

	var list apptest.DiscoverResponse
	res.Decode(t, &list)

	if len(list.Personas) != len(senders) {
		t.Fatalf("received %d personas, want %d", len(list.Personas), len(senders))
	}
	for i, got := range list.Personas {
		want := senderCards[len(senderCards)-1-i]
		if got.ID != want.ID {
			t.Fatalf("position %d: got %s, want %s (newest first)", i, got.ID, want.ID)
		}
	}
}

func TestOpeningReceivedLikesClearsTheBadge(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, _ := app.NewStartedClient(t)

	bob.MustLike(t, aliceCard.ID)

	if home := alice.Home(t); !home.HasUnseenLikes || home.ReceivedLikeCount != 1 {
		t.Fatalf("expected an unseen like: %+v", home)
	}

	alice.Do(t, http.MethodGet, "/api/likes/received", nil).RequireStatus(t, http.StatusOK)

	home := alice.Home(t)
	if home.HasUnseenLikes {
		t.Fatal("badge was not cleared after opening the screen")
	}
	if home.ReceivedLikeCount != 1 {
		t.Fatalf("received_like_count = %d, want 1", home.ReceivedLikeCount)
	}

	// A new like after the screen was opened raises the badge again.
	carol, _ := app.NewStartedClient(t)
	carol.MustLike(t, aliceCard.ID)
	if home := alice.Home(t); !home.HasUnseenLikes {
		t.Fatal("a new like did not raise the badge again")
	}
}

func TestSentLikesFlagsMatches(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)
	_, carolCard := app.NewStartedClient(t)

	alice.MustLike(t, carolCard.ID)
	alice.MustLike(t, bobCard.ID)
	bob.MustLike(t, aliceCard.ID)

	res := alice.Do(t, http.MethodGet, "/api/likes/sent", nil)
	res.RequireStatus(t, http.StatusOK)

	var list struct {
		Personas []struct {
			apptest.PersonaCard
			Matched bool `json:"matched"`
		} `json:"personas"`
	}
	res.Decode(t, &list)

	if len(list.Personas) != 2 {
		t.Fatalf("sent likes = %d, want 2", len(list.Personas))
	}
	if list.Personas[0].ID != bobCard.ID {
		t.Fatalf("newest first violated: %s", list.Personas[0].ID)
	}
	if !list.Personas[0].Matched {
		t.Fatal("matched target is missing the MATCH flag")
	}
	if list.Personas[1].Matched {
		t.Fatal("unmatched target is flagged as matched")
	}
}

func TestMatchesListsOnlyTheCounterpart(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)

	alice.MustLike(t, bobCard.ID)
	bob.MustLike(t, aliceCard.ID)

	res := alice.Do(t, http.MethodGet, "/api/matches", nil)
	res.RequireStatus(t, http.StatusOK)

	var list apptest.DiscoverResponse
	res.Decode(t, &list)

	if len(list.Personas) != 1 {
		t.Fatalf("matches = %d, want 1", len(list.Personas))
	}
	if list.Personas[0].ID != bobCard.ID {
		t.Fatalf("match list returned %s, want the counterpart %s", list.Personas[0].ID, bobCard.ID)
	}

	if home := alice.Home(t); home.HasUnseenMatches {
		t.Fatal("match badge was not cleared by opening the screen")
	}
}

func TestListsRequireAGeneratedPersona(t *testing.T) {
	app := apptest.New(t)
	newcomer := app.NewClient()
	newcomer.Home(t)

	for _, path := range []string{"/api/likes/received", "/api/likes/sent", "/api/matches"} {
		newcomer.Do(t, http.MethodGet, path, nil).RequireError(t, apperr.CodePersonaNotGenerated)
	}
}

func TestCleanupDeletesPreviousDaysOnly(t *testing.T) {
	app := apptest.New(t)
	alice, aliceCard := app.NewStartedClient(t)
	bob, bobCard := app.NewStartedClient(t)

	alice.MustLike(t, bobCard.ID)
	bob.MustLike(t, aliceCard.ID)

	// A third participant leaves a pass row behind, so every table has data.
	carol, _ := app.NewStartedClient(t)
	carol.MustPass(t, bobCard.ID)

	job := cleanup.NewJob(app.Pool, app.Clock)

	deleted, err := job.RunOnce(t.Context())
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("cleanup removed %d participants of the current day", deleted)
	}

	app.Clock.Advance(24 * time.Hour)

	deleted, err = job.RunOnce(t.Context())
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("cleanup removed %d participants, want 3", deleted)
	}

	for _, table := range []string{"participants", "personas", "likes", "passes", "matches"} {
		if got := app.CountRows(t, table); got != 0 {
			t.Fatalf("%s still has %d rows after cleanup", table, got)
		}
	}

	// Running again on an already clean database must be a no-op.
	deleted, err = job.RunOnce(t.Context())
	if err != nil {
		t.Fatalf("second cleanup: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("second cleanup removed %d rows, want 0", deleted)
	}
}

func TestCleanupKeepsTodayWhileDeletingYesterday(t *testing.T) {
	app := apptest.New(t)
	app.NewStartedClient(t)

	app.Clock.Advance(24 * time.Hour)
	today, _ := app.NewStartedClient(t)

	job := cleanup.NewJob(app.Pool, app.Clock)
	if _, err := job.RunOnce(t.Context()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if got := app.CountRows(t, "participants"); got != 1 {
		t.Fatalf("participants = %d, want 1", got)
	}

	home := today.Home(t)
	if !home.PersonaGenerated {
		t.Fatal("today's persona was deleted by the cleanup job")
	}
	if home.GameDate != clock.FormatGameDate(clock.Today(app.Clock)) {
		t.Fatalf("game_date = %s, want today", home.GameDate)
	}
}
