# learna-api developer tasks.
#
# On Windows these run under Git Bash or WSL. Without make, every recipe below
# is a plain go command you can copy and run directly.

.DEFAULT_GOAL := help

BINARY      := learna-api
BIN_DIR     := bin
CMD         := ./cmd/server
MIGRATE_DIR := internal/database/migrations

# Injected into the binary for /health to report. Falls back cleanly outside a
# git checkout.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# --- setup -------------------------------------------------------------------

.PHONY: setup
setup: ## First-time setup: create .env and resolve dependencies
	@test -f .env || (cp .env.example .env && echo "Created .env — set JWT_SECRET before running.")
	go mod tidy

.PHONY: deps
deps: ## Resolve and tidy module dependencies
	go mod tidy

# --- build & run -------------------------------------------------------------

.PHONY: build
build: ## Compile the server into bin/
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)

.PHONY: run
run: ## Run the server from source
	go run $(CMD)

.PHONY: clean
clean: ## Remove build artefacts
	rm -rf $(BIN_DIR)
	go clean -cache -testcache

# --- local run ---------------------------------------------------------------
#
# Each target sets APP_ENV, which is what selects the .env file:
#   development -> .env.development (API on 8081)
#   production  -> .env.production  (API on 8082)

.PHONY: dev
dev: ## Run the API in development on :8081
	APP_ENV=development go run $(CMD)

.PHONY: prod
prod: ## Run the API in production mode locally on :8082
	APP_ENV=production go run $(CMD)

# --- database (Neon) ---------------------------------------------------------

.PHONY: db-sync
db-sync: ## Apply pending migrations to the dev database
	APP_ENV=development go run $(CMD) -migrate=up

.PHONY: db-sync-prod
db-sync-prod: ## Apply pending migrations to the production database
	APP_ENV=production go run $(CMD) -migrate=up

.PHONY: db-status
db-status: ## Print the current schema version and dirty flag
	APP_ENV=development go run $(CMD) -migrate=version

# --- quality -----------------------------------------------------------------

.PHONY: fmt
fmt: ## Format all Go source
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (install: https://golangci-lint.run)
	golangci-lint run ./...

.PHONY: test
test: ## Run the test suite with the race detector
	go test -race ./...

.PHONY: cover
cover: ## Run tests and open the coverage report
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

.PHONY: check
check: fmt vet test ## Format, vet and test — run before pushing

# --- migrations --------------------------------------------------------------
#
# These go through the server binary rather than the golang-migrate CLI, so
# they read the same .env and the same embedded SQL the API itself uses.

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	go run $(CMD) -migrate=up

.PHONY: migrate-down
migrate-down: ## Roll back one migration (make migrate-down n=3 for more)
	go run $(CMD) -migrate=down -n=$(or $(n),1)

.PHONY: migrate-reset
migrate-reset: ## Roll back every migration — destroys all data
	go run $(CMD) -migrate=down -n=0

.PHONY: migrate-version
migrate-version: ## Print the current schema version
	go run $(CMD) -migrate=version

.PHONY: migrate-force
migrate-force: ## Force a version after a failed migration (make migrate-force n=1)
	go run $(CMD) -migrate=force -n=$(n)

.PHONY: migrate-new
migrate-new: ## Create an empty migration pair (make migrate-new name=add_quizzes)
	@test -n "$(name)" || (echo "usage: make migrate-new name=<snake_case_name>" && exit 1)
	@next=$$(printf "%06d" $$(( $$(ls $(MIGRATE_DIR)/*.up.sql 2>/dev/null | wc -l) + 1 ))); \
	touch $(MIGRATE_DIR)/$${next}_$(name).up.sql $(MIGRATE_DIR)/$${next}_$(name).down.sql; \
	echo "Created $(MIGRATE_DIR)/$${next}_$(name).{up,down}.sql"

# --- docker ------------------------------------------------------------------

.PHONY: docker-build
docker-build: ## Build the API image
	docker build -t $(BINARY):$(VERSION) -t $(BINARY):latest .

.PHONY: up
up: ## Start Postgres and the API
	docker compose up -d --build

.PHONY: down
down: ## Stop the stack
	docker compose down

.PHONY: down-v
down-v: ## Stop the stack and delete the database volume
	docker compose down -v

.PHONY: logs
logs: ## Tail the API logs
	docker compose logs -f api

.PHONY: db
db: ## Open psql against the compose database
	docker compose exec postgres psql -U $${DB_USER:-learna} -d $${DB_NAME:-learna}
