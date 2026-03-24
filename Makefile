PROJECT_NAME := $(shell basename $(PWD))
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

build: test clean
	@GOOS=${GOOS} GOARCH=${GOARCH} go build -o bin/${PROJECT_NAME} .

test:
	@go vet ./...
	@go test -cover ./...

clean:
	@go mod tidy

gosec:
	@gosec -terse ./...

lint:
	@golangci-lint run --timeout=2m

ready: test lint gosec

.PHONY: clean test build gosec lint ready
