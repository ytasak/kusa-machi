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
internal/db/         接続プール・マイグレーション実行
internal/db/query/   sqlc の入力 SQL
internal/db/sqlc/    sqlc 生成コード（手で編集しない）
internal/http/       chi ルータ・ハンドラ・ミドルウェア
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
| `COOKIE_SECURE` | `true` | 本番は必ず `true` |
| `COOKIE_SAMESITE` | `none` | kusa の iframe 埋め込みに必要。本番は `none` |
| `CLEANUP_INTERVAL` | `1h` | 前日データの物理削除ジョブの実行間隔 |
| `TEST_DATABASE_URL` | （未設定） | 設定時のみ DB 結合テストを実行 |

`COOKIE_SECURE=false` / `COOKIE_SAMESITE=lax` は **ローカル開発専用**。
`make run` はこの 2 つを自動で付ける。

## テスト

```bash
make test        # DB 不要のユニットテストのみ
make test-all    # 結合テストも実行（make db-up が必要）
```

## SQL を変更したとき

```bash
make migrate-new name=add_something   # migrations に up/down のペアを作る
make sqlc                             # internal/db/sqlc を再生成
```

`internal/db/sqlc/` は生成物なので手で編集しない。
