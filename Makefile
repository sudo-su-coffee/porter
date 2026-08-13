# Porter — build, run, test, deploy.
#
# Targets:
#   frontend  Build the Vue dashboard into backend/web/dist
#   backend   Build the Go control plane (cmd/server) -> backend/porter
#   build     frontend then backend (single binary)
#   run       build, then run ./backend/porter server
#   dev       Hint for the two-terminal dev loop (backend + Vite)
#   migrate   Run SQL migrations with golang-migrate
#   test      backend tests
#   clean     Remove build artifacts
#
# The Go binary embeds the built frontend via go:embed web/dist, so the
# frontend MUST be built first or `make backend` will embed an empty dist.

.PHONY: frontend backend build run dev migrate test validate clean

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo v0.1.0-beta-dev)

# Build the Vue 3 + Vite dashboard into backend/web/dist (source-only repo —
# no prebuilt bundle is checked in). Vite is configured to emit into
# ../backend/web/dist.
frontend:
	cd frontend && npm install && npm run build

# Build the Go binary. The single entrypoint is cmd/porter (the control-plane
# HTTP server with embedded workers; subcommands: `porter server|worker|kernel`).
backend:
	cd backend && go build -trimpath -o porter ./cmd/porter

# Full build: frontend then backend. The single binary ships the dashboard.
build: frontend backend

# Build, then run the server in the foreground. `server` is the default
# subcommand; pass extra flags through, e.g. `make run ARGS="-workers 4"`.
run: build
	./backend/porter server $(ARGS)

# Dev loop — run the two commands below in two separate terminals. Vite
# hot-reloads the dashboard and proxies /api to the Go backend on :8080.
dev:
	@echo "Porter dev loop — two terminals:"
	@echo ""
	@echo "  Terminal 1 (backend API):  cd backend && go run ./cmd/porter server"
	@echo "  Terminal 2 (frontend UI):  cd frontend && npm run dev"
	@echo ""
	@echo "Open http://localhost:5173 (Vite proxies /api -> :8080)"

validate:
	bash scripts/frontend/validate.sh

# Run pending SQL migrations in backend/migrations with golang-migrate.
# Uses $$PORTER_DATABASE_URL when set, else the default DSN below.
#   make migrate                       # default local DSN
#   PORTER_DATABASE_URL=... make migrate
MIGRATE ?= migrate
DB_URL ?= postgres://porter:porter@localhost:5432/porter?sslmode=disable
migrate:
	@DB="$${PORTER_DATABASE_URL:-$(DB_URL)}"; \
	$(MIGRATE) -path backend/migrations -database "$$DB" up

# Backend tests (the only tests in the repo, in backend/internal/compose).
test:
	cd backend && go test ./...

clean:
	rm -rf backend/porter backend/porter.sha256 backend/web/dist frontend/node_modules
