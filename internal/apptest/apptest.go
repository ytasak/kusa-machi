// Package apptest は本物のルータを本物の PostgreSQL に対して起動し、
// 操作可能な時計とともに API をエンドツーエンドで動かせるようにする。
//
// TEST_DATABASE_URL が設定されていない場合、テストはスキップされる
// （`make test-all` を参照）。
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

// App はテスト用データベースに接続した稼働中のサーバ。
type App struct {
	Pool   *pgxpool.Pool
	Clock  *clock.Fake
	Server *httptest.Server
	Photos *photo.Store
}

// New は時計を 2026-08-16 12:00 JST に固定し、データベースを空にした状態で
// サーバを起動する。
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
		// テストではフロントエンドのビルドを使わない。動かすのは /api だけ。
		WebDistDir: t.TempDir(),
	})

	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		pool.Close()
	})

	return &App{Pool: pool, Clock: clk, Server: srv, Photos: photos}
}

// Client は1つのブラウザ。独自の Cookie ジャーと日次 CSRF トークンを持つ。
type Client struct {
	app  *App
	http *http.Client
	csrf string
}

// NewClient はアプリに対する新しいブラウザセッションを開く。
func (a *App) NewClient() *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return &Client{app: a, http: &http.Client{Jar: jar}}
}

// DiscardCookies は Cookie を一切保存しないブラウザにする。
// サードパーティ Cookie を遮断する iOS Safari の再現に使う。
func (c *Client) DiscardCookies() { c.http.Jar = nil }

// Response はデコード済みの API レスポンス。
type Response struct {
	Status int
	Body   []byte
}

// Do はリクエストを実行する。更新系には保持している CSRF トークンを付ける。
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

// DoWithCSRF はトークンを明示して更新系リクエストを実行する。
// 偽造トークンや期限切れトークンの検証に使う。
func (c *Client) DoWithCSRF(t *testing.T, method, path string, body any, token string) *Response {
	t.Helper()
	saved := c.csrf
	c.csrf = token
	defer func() { c.csrf = saved }()
	return c.Do(t, method, path, body)
}

// CSRFToken はこのクライアントが現在持っているトークンを返す。
func (c *Client) CSRFToken() string { return c.csrf }

// SetCSRFToken は保持しているトークンを上書きする。
func (c *Client) SetCSRFToken(token string) { c.csrf = token }

// HomeResponse は GET /api/home のペイロードに対応する。
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

	ProfileRewardAvailable bool   `json:"profile_reward_available"`
	CSRFToken              string `json:"csrf_token"`
	CookieReceived         bool   `json:"cookie_received"`
}

// PersonaCard は公開 Persona のペイロードに対応する。
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

// Home はホーム画面を取得し、クライアントの CSRF トークンを更新する。
// 実際のフロントエンドが起動時に行うのと同じ動き。
func (c *Client) Home(t *testing.T) HomeResponse {
	t.Helper()
	res := c.Do(t, http.MethodGet, "/api/home", nil)
	res.RequireStatus(t, http.StatusOK)

	var home HomeResponse
	res.Decode(t, &home)
	c.csrf = home.CSRFToken
	return home
}

// GeneratePersona は「新しい人生を始める」を押す操作に相当する。
func (c *Client) GeneratePersona(t *testing.T) PersonaCard {
	t.Helper()
	res := c.Do(t, http.MethodPost, "/api/persona", nil)
	res.RequireStatus(t, http.StatusOK)

	var card PersonaCard
	res.Decode(t, &card)
	return card
}

// Start は共通の初期化。ホームを開き、当日の Persona を生成する。
func (c *Client) Start(t *testing.T) PersonaCard {
	t.Helper()
	c.Home(t)
	return c.GeneratePersona(t)
}

// NewStartedClient は当日の Persona をすでに持つブラウザを開く。
func (a *App) NewStartedClient(t *testing.T) (*Client, PersonaCard) {
	t.Helper()
	c := a.NewClient()
	return c, c.Start(t)
}

// NewStartedClients は Persona を持つ独立した participant を n 人作る。
func (a *App) NewStartedClients(t *testing.T, n int) ([]*Client, []PersonaCard) {
	t.Helper()
	clients := make([]*Client, n)
	cards := make([]PersonaCard, n)
	for i := range n {
		clients[i], cards[i] = a.NewStartedClient(t)
	}
	return clients, cards
}

// DiscoverResponse は GET /api/discover に対応する。
type DiscoverResponse struct {
	Personas []PersonaCard `json:"personas"`
}

// LikeResponse は POST /api/likes に対応する。
type LikeResponse struct {
	RemainingLikes int          `json:"remaining_likes"`
	Matched        bool         `json:"matched"`
	MatchID        *string      `json:"match_id"`
	LikesGained    int          `json:"likes_gained"`
	TargetPersona  *PersonaCard `json:"target_persona"`
}

// ProfileUpdateResponse は PATCH /api/persona/profile に対応する。
type ProfileUpdateResponse struct {
	Persona                PersonaCard `json:"persona"`
	RemainingLikes         int         `json:"remaining_likes"`
	LikesGained            int         `json:"likes_gained"`
	ProfileRewardAvailable bool        `json:"profile_reward_available"`
}

// PassResponse は POST /api/passes に対応する。
type PassResponse struct {
	PassCount        int  `json:"pass_count"`
	ExcludedForToday bool `json:"excluded_for_today"`
}

// Like は結果を検証せずに POST /api/likes を送る。
func (c *Client) Like(t *testing.T, personaID string) *Response {
	t.Helper()
	return c.Do(t, http.MethodPost, "/api/likes", map[string]any{"persona_id": personaID})
}

// Pass は結果を検証せずに POST /api/passes を送る。
func (c *Client) Pass(t *testing.T, personaID string) *Response {
	t.Helper()
	return c.Do(t, http.MethodPost, "/api/passes", map[string]any{"persona_id": personaID})
}

// MustLike は Like を送り、成功することを要求する。
func (c *Client) MustLike(t *testing.T, personaID string) LikeResponse {
	t.Helper()
	res := c.Like(t, personaID)
	res.RequireStatus(t, http.StatusOK)

	var out LikeResponse
	res.Decode(t, &out)
	return out
}

// MustPass は Pass を送り、成功することを要求する。
func (c *Client) MustPass(t *testing.T, personaID string) PassResponse {
	t.Helper()
	res := c.Pass(t, personaID)
	res.RequireStatus(t, http.StatusOK)

	var out PassResponse
	res.Decode(t, &out)
	return out
}

// UpdateProfile は結果を検証せずに B属性を保存する。
func (c *Client) UpdateProfile(t *testing.T, fields map[string]any) *Response {
	t.Helper()
	return c.Do(t, http.MethodPatch, "/api/persona/profile", fields)
}

// MustUpdateProfile は保存を送り、成功することを要求する。
func (c *Client) MustUpdateProfile(t *testing.T, fields map[string]any) ProfileUpdateResponse {
	t.Helper()
	res := c.UpdateProfile(t, fields)
	res.RequireStatus(t, http.StatusOK)

	var out ProfileUpdateResponse
	res.Decode(t, &out)
	return out
}

// CompleteProfile は3つの B属性をすべて埋める。プロフィール完成報酬の
// 条件をちょうど満たす操作。
func (c *Client) CompleteProfile(t *testing.T) ProfileUpdateResponse {
	t.Helper()
	return c.MustUpdateProfile(t, map[string]any{
		"name":  "さとし",
		"hobby": "散歩",
		"bio":   "よろしく",
	})
}

// Discover は候補のバッチを取得する。除外 id を指定することもできる。
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

// ExposureCount は Persona の exposure カウンタをデータベースから直接読む。
func (a *App) ExposureCount(t *testing.T, personaID string) int {
	t.Helper()
	var count int
	err := a.Pool.QueryRow(t.Context(), `SELECT exposure_count FROM personas WHERE id = $1`, personaID).Scan(&count)
	if err != nil {
		t.Fatalf("read exposure_count: %v", err)
	}
	return count
}

// RewardState は Like 回復の当日状態をデータベースから直接読む。
// API はこれらを公開しないので、二重付与の検証にはこの覗き穴を使う。
type RewardState struct {
	BonusLikes           int
	ProfileRewardClaimed bool
	MatchRewardCount     int
}

// RewardState は Persona の回復状態を返す。
func (a *App) RewardState(t *testing.T, personaID string) RewardState {
	t.Helper()
	var out RewardState
	err := a.Pool.QueryRow(t.Context(),
		`SELECT bonus_likes, profile_reward_claimed, match_reward_count
		   FROM personas WHERE id = $1`, personaID,
	).Scan(&out.BonusLikes, &out.ProfileRewardClaimed, &out.MatchRewardCount)
	if err != nil {
		t.Fatalf("read reward state: %v", err)
	}
	return out
}

// SpendLikes は n 回 Like を消費させる。回復の余地を作るための下準備。
// 相手はこのために用意した使い捨ての Persona で、Match は起きない。
func (a *App) SpendLikes(t *testing.T, spender *Client, n int) {
	t.Helper()
	_, targets := a.NewStartedClients(t, n)
	for _, target := range targets {
		spender.MustLike(t, target.ID)
	}
}

// CountRows はテーブルの全行を数える。API からは見えない不変条件の確認用。
func (a *App) CountRows(t *testing.T, table string) int {
	t.Helper()
	var count int
	if err := a.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

// Decode はボディを dst にアンマーシャルする。
func (r *Response) Decode(t *testing.T, dst any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, dst); err != nil {
		t.Fatalf("decode body %q: %v", r.Body, err)
	}
}

// ErrorCode はエラーレスポンスのドメインエラーコードを返す。
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

// RequireStatus はステータスが一致しなければテストを失敗させる。
func (r *Response) RequireStatus(t *testing.T, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status = %d, want %d (body: %s)", r.Status, want, r.Body)
	}
}

// RequireError はレスポンスが指定のコードを持ち、かつ API 契約が定める
// ステータスであることを要求する。
func (r *Response) RequireError(t *testing.T, want apperr.Code) {
	t.Helper()
	if got := r.ErrorCode(t); got != want {
		t.Fatalf("error code = %q, want %q (body: %s)", got, want, r.Body)
	}
	if want := apperr.HTTPStatus(want); r.Status != want {
		t.Fatalf("status = %d, want %d for %s", r.Status, want, r.Body)
	}
}
