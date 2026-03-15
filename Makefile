.PHONY: build proto lint test mock clean fmt

export GOEXPERIMENT := runtimesecret

build:
	go build -o bin/sekeve ./cmd/sekeve

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
