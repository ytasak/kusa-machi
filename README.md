# kusa-machi

匿名マッチングシミュレーションゲームの MVP。

毎日 00:00 JST にすべてのゲームデータがリセットされ、参加者はその日限りの
ランダム生成ペルソナで 10 Like を使い切るまで市場を回遊する。

仕様の Source of Truth は [`anonymous_matching_mvp_claude_handoff.md`](./anonymous_matching_mvp_claude_handoff.md)。

## 構成

| レイヤ | 採用技術 |
|---|---|
| Backend | Go 1.25 / chi |
| DB | PostgreSQL 17 |
| SQL | sqlc（ORM なし） |
| Migration | golang-migrate（バイナリに embed） |
| Frontend | Svelte 5 + Vite / CSS Modules |
| Deploy | 1 コンテナ（Go が `web/dist` を同一 Origin で配信） |

```text
cmd/server/          エントリポイント（serve / migrate up / migrate down）
internal/clock/      JST ゲーム日付の抽象化（テスト用 Fake クロック込み）
internal/config/     環境変数
internal/apperr/     ドメインエラーコードと HTTP ステータスの対応
internal/participant/ Cookie による匿名 identity と日次 Participant
internal/persona/    Persona 生成器とプロフィール入力の検証
internal/matching/   Like / Pass / Match のトランザクションと市場ルール
internal/photo/      プロフィール写真の正規化とファイル保存
internal/cleanup/    前日データの物理削除ジョブ
internal/db/         接続プール・マイグレーション実行
internal/db/query/   sqlc の入力 SQL
internal/db/sqlc/    sqlc 生成コード（手で編集しない）
internal/http/       chi ルータ・ハンドラ・ミドルウェア
internal/apptest/    実 DB と実ルータを使う結合テストの土台
migrations/          golang-migrate の SQL（embed される）
web/                 Svelte + Vite
```

## 必要なもの

- Go 1.25+
- Node.js 24+
- Docker（ローカル PostgreSQL 用）
- [sqlc](https://sqlc.dev/)（SQL を変更するときのみ）

## ローカル開発手順

```bash
# 1. DB を起動（localhost:5433、開発用の使い捨て認証情報）
make db-up

# 2. マイグレーション適用（サーバ起動時にも自動で走る）
make migrate-up

# 3. フロントエンドの依存をインストール
make web-install
```

### 開発中（フロントを HMR で回す）

ターミナルを 2 つ使う。

```bash
make run       # API を :8080 で起動
make web-dev   # Vite を :5173 で起動（/api を :8080 にプロキシ）
```

ブラウザは <http://localhost:5173> を開く。Vite のプロキシにより
ブラウザから見た Origin はフロントと API で同一になる。

### 本番と同じ構成で確認する

```bash
make web-build   # web/dist を生成
make run         # :8080 で API と web/dist を同一 Origin で配信
```

<http://localhost:8080> を開く。

### 1 コンテナでの起動

```bash
docker build -t kusa-machi .
docker run --rm -p 8080:8080 \
  -e DATABASE_URL='postgres://kusa:kusa@host.docker.internal:5433/kusamachi?sslmode=disable' \
  kusa-machi
```

## 環境変数

| 変数 | 既定値 | 説明 |
|---|---|---|
| `ADDR` | `:8080` | リッスンアドレス |
| `DATABASE_URL` | `postgres://kusa:kusa@localhost:5433/kusamachi?sslmode=disable` | 接続先 |
| `WEB_DIST_DIR` | `web/dist` | 配信するフロントエンドビルド |
| `PHOTO_DIR` | `data/photos` | プロフィール写真の保存先。`game_date` ごとのディレクトリに保存し、日次ジョブが前日分を削除する |
| `COOKIE_SECURE` | `true` | 本番は必ず `true` |
| `COOKIE_SAMESITE` | `none` | kusa の iframe 埋め込みに必要。本番は `none` |
| `CLEANUP_INTERVAL` | `1h` | 前日データの物理削除ジョブの実行間隔 |
| `TEST_DATABASE_URL` | （未設定） | 設定時のみ DB 結合テストを実行 |

`COOKIE_SECURE=false` / `COOKIE_SAMESITE=lax` は **ローカル開発専用**。
`make run` はこの 2 つを自動で付ける。

## API

すべて `/api` 配下、フロントエンドと同一 Origin。
更新系は `X-CSRF-Token` ヘッダ必須（トークンは `GET /api/home` が返す）。

| メソッド | パス | 用途 |
|---|---|---|
| GET | `/api/home` | 当日の Participant を保証し、ホーム画面の状態を返す |
| POST | `/api/persona` | 当日の Persona を生成（冪等・同日再生成なし） |
| GET | `/api/persona/me` | 自分の Persona |
| PATCH | `/api/persona/profile` | name / hobby / bio のみ更新 |
| POST | `/api/persona/photo` | プロフィール写真をアップロード（本文は生の画像） |
| DELETE | `/api/persona/photo` | プロフィール写真を削除 |
| GET | `/api/personas/{id}/photo` | 当日Personaの写真を配信 |
| GET | `/api/discover` | 最大5件の候補（`?exclude=id1,id2`） |
| POST | `/api/likes` | Like（10件/日、相互でMatch生成） |
| POST | `/api/passes` | Pass（同一相手3回で当日除外） |
| GET | `/api/likes/received` | Likeされた一覧（開くと既読化） |
| GET | `/api/likes/sent` | 送信済みLike一覧（`matched` フラグ付き） |
| GET | `/api/matches` | Match相手一覧（開くと既読化） |

エラーは `{"error":{"code":"LikeLimitExceeded","message":"..."}}` 形式。
フロントは `error.code` で分岐する。

## テスト

```bash
make test        # DB 不要のユニットテストのみ
make test-all    # 結合テストも実行（make db-up が必要）
```

結合テストは `TEST_DATABASE_URL` が設定されているときだけ実行され、
毎回テーブルを TRUNCATE してから実 DB と実ルータに対して動作する。
ゲーム日付は Fake クロックで進めるため、日付境界も実時間を待たずに検証できる。

重点的にカバーしているもの:

- Persona生成の全制約（年齢・学歴・職業・年収レンジ・10万円刻み）
- Like 10件上限 / 11件目の失敗 / 並行Like / 重複Like（二重消費なし）
- 相互Likeで Match が1件だけできること（同時実行含む）
- Pass 1→2→3 と当日除外、exposure_count の加算タイミング
- 日付境界（前日Personaは行動不可・前日データが当日に出ない）
- CSRF（不正トークンは拒否、前日トークンは DayExpired）
- プロフィール入力制約（文字数・改行・URL・A属性の拒否・HTMLはテキスト扱い）
- プロフィール写真（非画像の拒否・展開爆弾の拒否・EXIF除去・寸法上限・日次削除）

## SQL を変更したとき

```bash
make migrate-new name=add_something   # migrations に up/down のペアを作る
make sqlc                             # internal/db/sqlc を再生成
```

`internal/db/sqlc/` は生成物なので手で編集しない。
