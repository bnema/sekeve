.PHONY: build install proto lint test mock clean fmt wipe

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

wipe:
	@command -v jq >/dev/null 2>&1 || { echo "Error: jq is required but not installed."; exit 1; }
	@echo "WARNING: This will delete ALL entries in the vault."
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || { echo "Aborted."; exit 1; }
	@echo "Wiping all entries..."
	@sekeve list --json | jq -j '.[].name + "\u0000"' | while IFS= read -r -d '' name; do \
		sekeve rm "$$name" 2>/dev/null && echo "  deleted: $$name" || echo "  skipped: $$name"; \
	done
	@echo "Done."
