.PHONY: help db-up db-down db-reset migrate-up migrate-down migrate-new sqlc run build test test-all fmt web-install web-dev web-build

DATABASE_URL ?= postgres://kusa:kusa@localhost:5433/kusamachi?sslmode=disable
TEST_DATABASE_URL ?= postgres://kusa:kusa@localhost:5433/kusamachi_test?sslmode=disable

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

db-up: ## start the local PostgreSQL container
	docker compose up -d db

db-down: ## stop the local PostgreSQL container
	docker compose down

db-reset: ## drop the local database volume and start fresh
	docker compose down -v && docker compose up -d db

migrate-up: ## apply migrations
	DATABASE_URL='$(DATABASE_URL)' go run ./cmd/server migrate up

migrate-down: ## roll back all migrations
	DATABASE_URL='$(DATABASE_URL)' go run ./cmd/server migrate down

migrate-new: ## create a migration pair: make migrate-new name=add_something
	@test -n '$(name)' || (echo 'usage: make migrate-new name=add_something' && exit 1)
	@next=$$(printf '%06d' $$(( $$(ls migrations/*.up.sql 2>/dev/null | wc -l) + 1 ))); \
	touch "migrations/$${next}_$(name).up.sql" "migrations/$${next}_$(name).down.sql"; \
	echo "created migrations/$${next}_$(name).{up,down}.sql"

sqlc: ## regenerate internal/db/sqlc from migrations + queries
	sqlc generate

run: ## run the API server (serves web/dist at the same Origin)
	DATABASE_URL='$(DATABASE_URL)' COOKIE_SECURE=false COOKIE_SAMESITE=lax go run ./cmd/server

build: ## build the server binary into bin/
	go build -o bin/server ./cmd/server

test: ## run unit tests (no database required)
	go test ./...

test-all: ## run unit + integration tests against the test database
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test ./...

fmt: ## gofmt the tree
	gofmt -l -w .

web-install: ## install frontend dependencies
	npm --prefix web install

web-dev: ## run the Vite dev server (proxies /api to :8080)
	npm --prefix web run dev

web-build: ## build the frontend into web/dist
	npm --prefix web run build
