package handler_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"kusamachi/internal/apperr"
	"kusamachi/internal/apptest"
	"kusamachi/internal/matching"
	"kusamachi/internal/participant"
)

func TestHomeCreatesParticipantAndIssuesCookie(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()

	home := c.Home(t)

	if home.GameDate != "2026-08-16" {
		t.Fatalf("game_date = %q, want 2026-08-16", home.GameDate)
	}
	if !strings.HasSuffix(home.ServerTime, "+09:00") {
		t.Fatalf("server_time %q is not JST", home.ServerTime)
	}
	if home.PersonaGenerated || home.Persona != nil {
		t.Fatalf("expected no persona on first access, got %+v", home.Persona)
	}
	if home.RemainingLikes != matching.DailyLikeBudget {
		t.Fatalf("remaining_likes = %d, want %d", home.RemainingLikes, matching.DailyLikeBudget)
	}
	if home.CSRFToken == "" {
		t.Fatal("csrf_token is empty")
	}
}

func TestCookieIsIssuedHttpOnly(t *testing.T) {
	app := apptest.New(t)

	res, err := http.Get(app.Server.URL + "/api/home")
	if err != nil {
		t.Fatalf("GET /api/home: %v", err)
	}
	defer res.Body.Close()

	var found *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == participant.CookieName {
			found = c
		}
	}
	if found == nil {
		t.Fatalf("cookie %q was not issued", participant.CookieName)
	}
	if !found.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
	if found.Path != "/" {
		t.Errorf("cookie path = %q, want /", found.Path)
	}
	if found.Domain != "" {
		t.Errorf("cookie must not set an explicit domain, got %q", found.Domain)
	}
	if found.MaxAge < 29*24*3600 || found.MaxAge > 31*24*3600 {
		t.Errorf("cookie max-age = %d, want about 30 days", found.MaxAge)
	}
}

func TestSameCookieKeepsSameParticipantAndCSRFToken(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()

	first := c.Home(t)
	second := c.Home(t)

	if first.CSRFToken != second.CSRFToken {
		t.Fatal("csrf token changed within the same game day")
	}
}

func TestGeneratePersonaIsIdempotent(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()

	first := c.Start(t)
	second := c.GeneratePersona(t)

	if first.ID != second.ID {
		t.Fatalf("persona was rerolled: %s -> %s", first.ID, second.ID)
	}
	if first != second {
		t.Fatalf("persona attributes changed: %+v -> %+v", first, second)
	}
}

func TestConcurrentGeneratePersonaReturnsOnePersona(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()
	c.Home(t)

	const attempts = 8
	ids := make([]string, attempts)

	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := c.Do(t, http.MethodPost, "/api/persona", nil)
			if res.Status != http.StatusOK {
				return
			}
			var card apptest.PersonaCard
			res.Decode(t, &card)
			ids[i] = card.ID
		}()
	}
	wg.Wait()

	for i, id := range ids {
		if id == "" {
			t.Fatalf("attempt %d did not return a persona", i)
		}
		if id != ids[0] {
			t.Fatalf("concurrent generation produced different personas: %s vs %s", ids[0], id)
		}
	}

	var count int
	if err := app.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM personas`).Scan(&count); err != nil {
		t.Fatalf("count personas: %v", err)
	}
	if count != 1 {
		t.Fatalf("persona rows = %d, want 1", count)
	}
}

func TestPersonaMeRequiresGeneration(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()
	c.Home(t)

	res := c.Do(t, http.MethodGet, "/api/persona/me", nil)
	res.RequireError(t, apperr.CodePersonaNotGenerated)

	c.GeneratePersona(t)

	res = c.Do(t, http.MethodGet, "/api/persona/me", nil)
	res.RequireStatus(t, http.StatusOK)
}

func TestProfileUpdate(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()
	c.Start(t)

	res := c.Do(t, http.MethodPatch, "/api/persona/profile", map[string]any{
		"name":  "  さとし  ",
		"hobby": "散歩",
		"bio":   "よろしく",
	})
	res.RequireStatus(t, http.StatusOK)

	var card apptest.PersonaCard
	res.Decode(t, &card)
	if card.Name == nil || *card.Name != "さとし" {
		t.Fatalf("name = %v, want trimmed さとし", card.Name)
	}

	// Blank values clear the field so the card can omit the row entirely.
	res = c.Do(t, http.MethodPatch, "/api/persona/profile", map[string]any{"name": "   "})
	res.RequireStatus(t, http.StatusOK)

	var cleared apptest.PersonaCard
	res.Decode(t, &cleared)
	if cleared.Name != nil || cleared.Hobby != nil || cleared.Bio != nil {
		t.Fatalf("expected all B fields cleared, got %+v", cleared)
	}

	if !strings.Contains(string(res.Body), `"age"`) {
		t.Fatal("A attributes should still be present on the card")
	}
	if strings.Contains(string(res.Body), `"name"`) {
		t.Fatal("unset fields must be omitted from the card")
	}
}

func TestProfileRejectsInvalidInput(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()
	c.Start(t)

	cases := []struct {
		name string
		body map[string]any
		code apperr.Code
	}{
		{"newline", map[string]any{"bio": "あ\nい"}, apperr.CodeInvalidProfileInput},
		{"url", map[string]any{"bio": "https://example.com"}, apperr.CodeInvalidProfileInput},
		{"too long", map[string]any{"name": strings.Repeat("あ", 21)}, apperr.CodeInvalidProfileInput},
		{"system attribute", map[string]any{"age": 20}, apperr.CodeInvalidRequest},
		{"system attribute with valid field", map[string]any{"name": "x", "annual_income": 9999}, apperr.CodeInvalidRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := c.Do(t, http.MethodPatch, "/api/persona/profile", tc.body)
			res.RequireError(t, tc.code)
		})
	}

	// The persona's A attributes must be untouched by the rejected payloads.
	res := c.Do(t, http.MethodGet, "/api/persona/me", nil)
	res.RequireStatus(t, http.StatusOK)
	var card apptest.PersonaCard
	res.Decode(t, &card)
	if card.Age < 20 || card.Age > 50 {
		t.Fatalf("age was corrupted: %d", card.Age)
	}
}

func TestHTMLInProfileIsStoredAsText(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()
	c.Start(t)

	const payload = "<script>alert(1)</script>"
	res := c.Do(t, http.MethodPatch, "/api/persona/profile", map[string]any{"bio": payload})
	res.RequireStatus(t, http.StatusOK)

	var card apptest.PersonaCard
	res.Decode(t, &card)
	if card.Bio == nil || *card.Bio != payload {
		t.Fatalf("bio = %v, want the raw text preserved", card.Bio)
	}
	// encoding/json escapes < and > so the payload can never break out of a
	// script context on the client side.
	if strings.Contains(string(res.Body), "<script>") {
		t.Fatalf("raw html leaked into the JSON body: %s", res.Body)
	}
}

func TestCSRFIsRequiredForMutations(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()
	c.Home(t)

	res := c.DoWithCSRF(t, http.MethodPost, "/api/persona", nil, "")
	res.RequireError(t, apperr.CodeInvalidCSRF)

	res = c.DoWithCSRF(t, http.MethodPost, "/api/persona", nil, "not-the-token")
	res.RequireError(t, apperr.CodeInvalidCSRF)

	// A rejected mutation must not have created anything.
	var count int
	if err := app.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM personas`).Scan(&count); err != nil {
		t.Fatalf("count personas: %v", err)
	}
	if count != 0 {
		t.Fatalf("persona rows = %d, want 0", count)
	}

	res = c.Do(t, http.MethodPost, "/api/persona", nil)
	res.RequireStatus(t, http.StatusOK)
}

func TestYesterdaysCSRFTokenReportsDayExpired(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()
	c.Start(t)
	yesterdayToken := c.CSRFToken()

	app.Clock.Advance(24 * time.Hour)

	// The tab is still open: it holds yesterday's token and does not know the
	// day rolled over.
	res := c.DoWithCSRF(t, http.MethodPatch, "/api/persona/profile", map[string]any{"name": "x"}, yesterdayToken)
	res.RequireError(t, apperr.CodeDayExpired)
}

func TestNewGameDayStartsANewLife(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()
	yesterday := c.Start(t)

	app.Clock.Set(app.Clock.Now().Add(12 * time.Hour)) // crosses 00:00 JST

	home := c.Home(t)
	if home.GameDate != "2026-08-17" {
		t.Fatalf("game_date = %q, want 2026-08-17", home.GameDate)
	}
	if home.PersonaGenerated || home.Persona != nil {
		t.Fatalf("yesterday's persona leaked into the new day: %+v", home.Persona)
	}

	today := c.GeneratePersona(t)
	if today.ID == yesterday.ID {
		t.Fatal("the previous day's persona was reused")
	}

	// Both participants exist until the cleanup job removes the old one.
	var participants int
	if err := app.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM participants`).Scan(&participants); err != nil {
		t.Fatalf("count participants: %v", err)
	}
	if participants != 2 {
		t.Fatalf("participant rows = %d, want 2 (one per game day)", participants)
	}
}
