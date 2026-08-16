.PHONY: help db-up db-down db-reset migrate-up migrate-down migrate-new sqlc run build test test-all fmt web-install web-dev web-build

DATABASE_URL ?= postgres://kusa:kusa@localhost:5433/kusamachi?sslmode=disable
TEST_DATABASE_URL ?= postgres://kusa:kusa@localhost:5433/kusamachi_test?sslmode=disable

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

db-up: ## ローカルの PostgreSQL コンテナを起動する
	docker compose up -d db

db-down: ## ローカルの PostgreSQL コンテナを停止する
	docker compose down

db-reset: ## ローカル DB のボリュームを破棄して作り直す
	docker compose down -v && docker compose up -d db

migrate-up: ## マイグレーションを適用する
	DATABASE_URL='$(DATABASE_URL)' go run ./cmd/server migrate up

migrate-down: ## マイグレーションをすべて巻き戻す
	DATABASE_URL='$(DATABASE_URL)' go run ./cmd/server migrate down

migrate-new: ## マイグレーションの up/down を作る: make migrate-new name=add_something
	@test -n '$(name)' || (echo '使い方: make migrate-new name=add_something' && exit 1)
	@next=$$(printf '%06d' $$(( $$(ls migrations/*.up.sql 2>/dev/null | wc -l) + 1 ))); \
	touch "migrations/$${next}_$(name).up.sql" "migrations/$${next}_$(name).down.sql"; \
	echo "作成: migrations/$${next}_$(name).{up,down}.sql"

sqlc: ## migrations とクエリから internal/db/sqlc を再生成する
	sqlc generate

run: ## API サーバを起動する（web/dist を同一 Origin で配信）
	DATABASE_URL='$(DATABASE_URL)' COOKIE_SECURE=false COOKIE_SAMESITE=lax go run ./cmd/server

build: ## サーバのバイナリを bin/ にビルドする
	go build -o bin/server ./cmd/server

test: ## ユニットテストを実行する（DB 不要）
	go test ./...

test-all: ## テスト用 DB に対して結合テストも含めて実行する
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test ./...

fmt: ## ツリー全体に gofmt をかける
	gofmt -l -w .

web-install: ## フロントエンドの依存をインストールする
	npm --prefix web install

web-dev: ## Vite の開発サーバを起動する（/api を :8080 にプロキシ）
	npm --prefix web run dev

web-build: ## フロントエンドを web/dist にビルドする
	npm --prefix web run build
