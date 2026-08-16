// Package apptest boots the real router against a real PostgreSQL database so
// the API can be exercised end to end, with a controllable clock.
//
// Tests are skipped unless TEST_DATABASE_URL is set (see `make test-all`).
package apptest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"kusamachi/internal/apperr"
	"kusamachi/internal/clock"
	"kusamachi/internal/db"
	httpx "kusamachi/internal/http"
	"kusamachi/internal/http/middleware"
	"kusamachi/internal/participant"
	"kusamachi/internal/persona"
	"kusamachi/internal/photo"
)

const envTestDatabaseURL = "TEST_DATABASE_URL"

var migrateOnce sync.Once

// App is a running server backed by the test database.
type App struct {
	Pool   *pgxpool.Pool
	Clock  *clock.Fake
	Server *httptest.Server
	Photos *photo.Store
}

// New starts a server whose clock is pinned to 2026-08-16 12:00 JST and whose
// database has been emptied.
func New(t *testing.T) *App {
	t.Helper()

	dsn := os.Getenv(envTestDatabaseURL)
	if dsn == "" {
		t.Skipf("%s is not set; skipping database-backed test", envTestDatabaseURL)
	}

	var migrateErr error
	migrateOnce.Do(func() { migrateErr = db.Migrate(dsn) })
	if migrateErr != nil {
		t.Fatalf("migrate test database: %v", migrateErr)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}

	if _, err := pool.Exec(ctx, `TRUNCATE participants, personas, likes, passes, matches`); err != nil {
		pool.Close()
		t.Fatalf("truncate: %v", err)
	}

	clk := clock.NewFakeJST(2026, time.August, 16, 12, 0, 0)

	photos, err := photo.NewStore(t.TempDir())
	if err != nil {
		pool.Close()
		t.Fatalf("photo store: %v", err)
	}

	router := httpx.NewRouter(httpx.Deps{
		Pool:      pool,
		Clock:     clk,
		Generator: persona.NewGenerator(),
		Photos:    photos,
		Cookie: participant.CookieConfig{
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
		},
		// No frontend build in tests; only /api is exercised.
		WebDistDir: t.TempDir(),
	})

	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		pool.Close()
	})

	return &App{Pool: pool, Clock: clk, Server: srv, Photos: photos}
}

// Client is one browser: its own cookie jar and its own daily CSRF token.
type Client struct {
	app  *App
	http *http.Client
	csrf string
}

// NewClient opens a fresh browser session against the app.
func (a *App) NewClient() *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return &Client{app: a, http: &http.Client{Jar: jar}}
}

// Response is a decoded API response.
type Response struct {
	Status int
	Body   []byte
}

// Do performs a request, attaching the stored CSRF token to mutating calls.
func (c *Client) Do(t *testing.T, method, path string, body any) *Response {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, c.app.Server.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set(middleware.CSRFHeader, c.csrf)
	}

	res, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return &Response{Status: res.StatusCode, Body: raw}
}

// DoWithCSRF performs a mutating request with an explicit token, for testing
// forged and stale tokens.
func (c *Client) DoWithCSRF(t *testing.T, method, path string, body any, token string) *Response {
	t.Helper()
	saved := c.csrf
	c.csrf = token
	defer func() { c.csrf = saved }()
	return c.Do(t, method, path, body)
}

// CSRFToken exposes the token this client currently holds.
func (c *Client) CSRFToken() string { return c.csrf }

// SetCSRFToken overrides the stored token.
func (c *Client) SetCSRFToken(token string) { c.csrf = token }

// HomeResponse mirrors the GET /api/home payload.
type HomeResponse struct {
	ServerTime        string       `json:"server_time"`
	GameDate          string       `json:"game_date"`
	PersonaGenerated  bool         `json:"persona_generated"`
	Persona           *PersonaCard `json:"persona"`
	RemainingLikes    int          `json:"remaining_likes"`
	ReceivedLikeCount int64        `json:"received_like_count"`
	MatchCount        int64        `json:"match_count"`
	HasUnseenLikes    bool         `json:"has_unseen_likes"`
	HasUnseenMatches  bool         `json:"has_unseen_matches"`
	CSRFToken         string       `json:"csrf_token"`
}

// PersonaCard mirrors the public persona payload.
type PersonaCard struct {
	ID           string  `json:"id"`
	Name         *string `json:"name"`
	Age          int     `json:"age"`
	Gender       string  `json:"gender"`
	HeightCm     int     `json:"height_cm"`
	Occupation   string  `json:"occupation"`
	AnnualIncome int     `json:"annual_income"`
	Education    string  `json:"education"`
	Hobby        *string `json:"hobby"`
	Bio          *string `json:"bio"`
}

// Home fetches the home screen and refreshes the client's CSRF token, which is
// what the real frontend does on start-up.
func (c *Client) Home(t *testing.T) HomeResponse {
	t.Helper()
	res := c.Do(t, http.MethodGet, "/api/home", nil)
	res.RequireStatus(t, http.StatusOK)

	var home HomeResponse
	res.Decode(t, &home)
	c.csrf = home.CSRFToken
	return home
}

// GeneratePersona presses "新しい人生を始める".
func (c *Client) GeneratePersona(t *testing.T) PersonaCard {
	t.Helper()
	res := c.Do(t, http.MethodPost, "/api/persona", nil)
	res.RequireStatus(t, http.StatusOK)

	var card PersonaCard
	res.Decode(t, &card)
	return card
}

// Start is the common bootstrap: open home, then generate today's persona.
func (c *Client) Start(t *testing.T) PersonaCard {
	t.Helper()
	c.Home(t)
	return c.GeneratePersona(t)
}

// NewStartedClient opens a browser that already has today's persona.
func (a *App) NewStartedClient(t *testing.T) (*Client, PersonaCard) {
	t.Helper()
	c := a.NewClient()
	return c, c.Start(t)
}

// NewStartedClients creates n independent participants with personas.
func (a *App) NewStartedClients(t *testing.T, n int) ([]*Client, []PersonaCard) {
	t.Helper()
	clients := make([]*Client, n)
	cards := make([]PersonaCard, n)
	for i := range n {
		clients[i], cards[i] = a.NewStartedClient(t)
	}
	return clients, cards
}

// DiscoverResponse mirrors GET /api/discover.
type DiscoverResponse struct {
	Personas []PersonaCard `json:"personas"`
}

// LikeResponse mirrors POST /api/likes.
type LikeResponse struct {
	RemainingLikes int          `json:"remaining_likes"`
	Matched        bool         `json:"matched"`
	MatchID        *string      `json:"match_id"`
	TargetPersona  *PersonaCard `json:"target_persona"`
}

// PassResponse mirrors POST /api/passes.
type PassResponse struct {
	PassCount        int  `json:"pass_count"`
	ExcludedForToday bool `json:"excluded_for_today"`
}

// Like sends POST /api/likes without asserting the outcome.
func (c *Client) Like(t *testing.T, personaID string) *Response {
	t.Helper()
	return c.Do(t, http.MethodPost, "/api/likes", map[string]any{"persona_id": personaID})
}

// Pass sends POST /api/passes without asserting the outcome.
func (c *Client) Pass(t *testing.T, personaID string) *Response {
	t.Helper()
	return c.Do(t, http.MethodPost, "/api/passes", map[string]any{"persona_id": personaID})
}

// MustLike sends a like and requires it to succeed.
func (c *Client) MustLike(t *testing.T, personaID string) LikeResponse {
	t.Helper()
	res := c.Like(t, personaID)
	res.RequireStatus(t, http.StatusOK)

	var out LikeResponse
	res.Decode(t, &out)
	return out
}

// MustPass sends a pass and requires it to succeed.
func (c *Client) MustPass(t *testing.T, personaID string) PassResponse {
	t.Helper()
	res := c.Pass(t, personaID)
	res.RequireStatus(t, http.StatusOK)

	var out PassResponse
	res.Decode(t, &out)
	return out
}

// Discover fetches a candidate batch, optionally excluding ids.
func (c *Client) Discover(t *testing.T, exclude ...string) DiscoverResponse {
	t.Helper()
	path := "/api/discover"
	if len(exclude) > 0 {
		path += "?exclude=" + strings.Join(exclude, ",")
	}

	res := c.Do(t, http.MethodGet, path, nil)
	res.RequireStatus(t, http.StatusOK)

	var out DiscoverResponse
	res.Decode(t, &out)
	return out
}

// ExposureCount reads a persona's exposure counter straight from the database.
func (a *App) ExposureCount(t *testing.T, personaID string) int {
	t.Helper()
	var count int
	err := a.Pool.QueryRow(t.Context(), `SELECT exposure_count FROM personas WHERE id = $1`, personaID).Scan(&count)
	if err != nil {
		t.Fatalf("read exposure_count: %v", err)
	}
	return count
}

// CountRows counts every row of a table, for invariants the API cannot show.
func (a *App) CountRows(t *testing.T, table string) int {
	t.Helper()
	var count int
	if err := a.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

// Decode unmarshals the body into dst.
func (r *Response) Decode(t *testing.T, dst any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, dst); err != nil {
		t.Fatalf("decode body %q: %v", r.Body, err)
	}
}

// ErrorCode returns the domain error code of an error response.
func (r *Response) ErrorCode(t *testing.T) apperr.Code {
	t.Helper()
	var envelope struct {
		Error struct {
			Code apperr.Code `json:"code"`
		} `json:"error"`
	}
	r.Decode(t, &envelope)
	return envelope.Error.Code
}

// RequireStatus fails the test unless the status matches.
func (r *Response) RequireStatus(t *testing.T, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status = %d, want %d (body: %s)", r.Status, want, r.Body)
	}
}

// RequireError fails the test unless the response carries the given code with
// the status the API contract maps it to.
func (r *Response) RequireError(t *testing.T, want apperr.Code) {
	t.Helper()
	if got := r.ErrorCode(t); got != want {
		t.Fatalf("error code = %q, want %q (body: %s)", got, want, r.Body)
	}
	if want := apperr.HTTPStatus(want); r.Status != want {
		t.Fatalf("status = %d, want %d for %s", r.Status, want, r.Body)
	}
}
