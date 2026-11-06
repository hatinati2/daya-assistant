.PHONY: test build run fmt vet

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/daya-assistant ./cmd/server

run:
	go run ./cmd/server

fmt:
	gofmt -w ./cmd
	gofmt -w ./internal

vet:
	go vet ./...
