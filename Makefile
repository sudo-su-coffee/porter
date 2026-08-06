.PHONY: frontend backend build run dev clean

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo v0.1.0-beta-dev)

# Build the Vue dashboard into backend/web/dist (source-only repo — no
# prebuilt bundle is checked in).
frontend:
	cd frontend && npm install && npm run build

# Build the Go binary. Assumes `make frontend` has already populated
# backend/web/dist, since the assets package embeds it via go:embed.
# The main package lives at cmd/porter.
backend:
	cd backend && go build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" -o porter ./cmd/porter

# Full build: frontend then backend, single binary out at backend/porter
build: frontend backend

# Build and run in the foreground
run: build
	cd backend && ./porter

# Two-terminal-in-one dev loop reminder — Vite hot-reloads the dashboard
# and proxies API calls to a `go run ./cmd/porter` backend. Run these in
# two separate terminals; this target just starts the backend.
dev:
	cd backend && go run ./cmd/porter

clean:
	rm -f backend/porter backend/porter.sha256 backend/porter.db
	rm -rf backend/web/dist/assets backend/web/dist/index.html
	rm -rf frontend/node_modules
