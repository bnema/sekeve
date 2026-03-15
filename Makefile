.PHONY: build proto lint test mock clean fmt

build:
	GOEXPERIMENT=runtimesecret go build -o bin/sekeve ./cmd/sekeve

proto:
	cd proto && buf generate

lint:
	golangci-lint run ./...

test:
	GOEXPERIMENT=runtimesecret go test ./...

mock:
	mockery

fmt:
	go fmt ./...

clean:
	rm -rf bin/
