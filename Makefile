BINARY_NAME = sogark
MODULE = github.com/sogei/cyberark-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

.PHONY: build install test clean build-all

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/sogark

install:
	go install $(LDFLAGS) ./cmd/sogark

test:
	go test ./...

clean:
	rm -rf bin/

build-all: clean
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64  ./cmd/sogark
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64  ./cmd/sogark
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64   ./cmd/sogark
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/sogark
