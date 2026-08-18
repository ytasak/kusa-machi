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

// フロントエンドは cookie_received を見て、ブラウザが Cookie を保存して
// いないこと（＝サードパーティ遮断で遊べない状態）を判定する。
func TestHomeReportsWhetherTheCookieCameBack(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()

	if first := c.Home(t); first.CookieReceived {
		t.Fatal("cookie_received must be false on the very first request")
	}
	if second := c.Home(t); !second.CookieReceived {
		t.Fatal("cookie_received must be true once the client sends the cookie back")
	}

	// Cookie を保存しないブラウザの再現。毎回 false のままになる。
	blocked := app.NewClient()
	blocked.DiscardCookies()
	for i := range 2 {
		if home := blocked.Home(t); home.CookieReceived {
			t.Fatalf("request %d: cookie_received must stay false when cookies are dropped", i+1)
		}
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

	var saved apptest.ProfileUpdateResponse
	res.Decode(t, &saved)
	if saved.Persona.Name == nil || *saved.Persona.Name != "さとし" {
		t.Fatalf("name = %v, want trimmed さとし", saved.Persona.Name)
	}

	// 空白だけの値はフィールドをクリアし、カードから項目ごと消えるようにする。
	res = c.Do(t, http.MethodPatch, "/api/persona/profile", map[string]any{"name": "   "})
	res.RequireStatus(t, http.StatusOK)

	var cleared apptest.ProfileUpdateResponse
	res.Decode(t, &cleared)
	if cleared.Persona.Name != nil || cleared.Persona.Hobby != nil || cleared.Persona.Bio != nil {
		t.Fatalf("expected all B fields cleared, got %+v", cleared.Persona)
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
		{"改行", map[string]any{"bio": "あ\nい"}, apperr.CodeInvalidProfileInput},
		{"URL", map[string]any{"bio": "https://example.com"}, apperr.CodeInvalidProfileInput},
		{"文字数超過", map[string]any{"name": strings.Repeat("あ", 21)}, apperr.CodeInvalidProfileInput},
		{"システム生成属性", map[string]any{"age": 20}, apperr.CodeInvalidRequest},
		{"正当な項目に混ぜたシステム生成属性", map[string]any{"name": "x", "annual_income": 9999}, apperr.CodeInvalidRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := c.Do(t, http.MethodPatch, "/api/persona/profile", tc.body)
			res.RequireError(t, tc.code)
		})
	}

	// 拒否されたペイロードによって Persona の A属性が変わってはいけない。
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

	var saved apptest.ProfileUpdateResponse
	res.Decode(t, &saved)
	if saved.Persona.Bio == nil || *saved.Persona.Bio != payload {
		t.Fatalf("bio = %v, want the raw text preserved", saved.Persona.Bio)
	}
	// encoding/json が < と > をエスケープするため、ペイロードがクライアント側の
	// script コンテキストを抜け出すことはない。
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

	// 拒否された更新は何も作ってはいけない。
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

	// タブは開きっぱなしで前日のトークンを持ったまま、日付が変わったことを
	// 知らない状態。
	res := c.DoWithCSRF(t, http.MethodPatch, "/api/persona/profile", map[string]any{"name": "x"}, yesterdayToken)
	res.RequireError(t, apperr.CodeDayExpired)
}

func TestNewGameDayStartsANewLife(t *testing.T) {
	app := apptest.New(t)
	c := app.NewClient()
	yesterday := c.Start(t)

	app.Clock.Set(app.Clock.Now().Add(12 * time.Hour)) // JST の 00:00 をまたぐ

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

	// クリーンアップジョブが古い方を消すまで、両方の participant が存在する。
	var participants int
	if err := app.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM participants`).Scan(&participants); err != nil {
		t.Fatalf("count participants: %v", err)
	}
	if participants != 2 {
		t.Fatalf("participant rows = %d, want 2 (one per game day)", participants)
	}
}
