# Porter backend — build, run, dev, test, clean (self-contained; no need to cd
# to the repo root). The binary embeds web/dist, so build the frontend first:
#   make frontend    (npm install + vite build -> web/dist)
#   make build
#
# Entrypoint is cmd/porter (`porter server|worker|kernel|version`).

.PHONY: frontend build run dev test vet clean

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo v0.1.0-beta-dev)
DB_URL  ?= postgres://porter:porter@localhost:5432/porter?sslmode=disable

frontend:
	cd ../frontend && npm install && npm run build

build:
	go build -trimpath -ldflags "-X main.Version=$(VERSION)" -o bin/porter ./cmd/porter

run: build
	./bin/porter server $(ARGS)

# Local dev — one-command dev environment (Docker Postgres + simulate mode).
dev:
	bash ../scripts/backend/dev.sh up

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin porter porter.db
