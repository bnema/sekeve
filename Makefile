.PHONY: build proto lint test mock clean fmt

export GOEXPERIMENT := runtimesecret

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/bnema/sekeve/internal/version.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/sekeve ./cmd/sekeve

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
