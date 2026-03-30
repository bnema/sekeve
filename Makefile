.PHONY: build install proto lint test mock clean fmt wipe server-reset server-init server-rebuild

export GOEXPERIMENT := runtimesecret

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/bnema/sekeve/internal/version.Version=$(VERSION)
build:
	go build -ldflags "$(LDFLAGS)" -o bin/sekeve ./cmd/sekeve

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/sekeve

proto:
	cd proto && buf generate

lint:
	golangci-lint run ./...

test:
	go test ./...

mock:
	mockery

fmt:
	go fmt ./...

clean:
	rm -rf bin/

# Set GPG_KEY to your GPG key ID (e.g., GPG_KEY=you@example.com make server-init)
GPG_KEY ?=

wipe:
	@command -v jq >/dev/null 2>&1 || { echo "Error: jq is required but not installed."; exit 1; }
	@echo "WARNING: This will delete ALL entries in the vault."
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || { echo "Aborted."; exit 1; }
	@echo "Wiping all entries..."
	@sekeve list --json | jq -j '.[].id + "\u0000"' | while IFS= read -r -d '' id; do \
		sekeve rm --id "$$id" 2>/dev/null && echo "  deleted: $$id" || echo "  skipped: $$id"; \
	done
	@echo "Done."

# Init the server GPG key inside the container (requires running container)
server-init:
	@[ -n "$(GPG_KEY)" ] || { echo "Error: GPG_KEY is required (e.g., GPG_KEY=you@example.com make server-init)"; exit 1; }
	gpg --export --armor $(GPG_KEY) | docker compose run -T sekeve-server server init --data /data/sekeve.db

# Rebuild image, wipe volume, re-init GPG key and restart
server-reset:
	@echo "WARNING: This will destroy the server database and all entries."
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || { echo "Aborted."; exit 1; }
	docker compose down -v --remove-orphans
	rm -f $(XDG_CONFIG_HOME)/sekeve/session $(HOME)/.config/sekeve/session 2>/dev/null; true
	docker compose build
	docker compose up -d
	@echo "Initializing GPG key..."
	$(MAKE) server-init
	docker compose down --remove-orphans
	docker compose up -d
	@echo "Waiting for server to be healthy..."
	@sleep 3
	@docker compose ps
	@echo "Server reset complete. Ready for import."

# Rebuild image and restart without wiping data
server-rebuild:
	docker compose build
	docker compose up -d
