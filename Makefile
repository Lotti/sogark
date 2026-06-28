BINARY_NAME = sogark
MODULE = github.com/sogei/cyberark-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

.PHONY: build install test clean build-all release

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
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64   ./cmd/sogark
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/sogark
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-arm64.exe ./cmd/sogark

# --- svu versioning ---
# svu is a tiny Go binary for semantic versioning from conventional commits.
# Install: go install github.com/caarlos0/svu/v3@latest

.PHONY: version

version:
	@svu current 2>/dev/null || echo "dev"

next-version:
	@svu next 2>/dev/null || echo "v0.1.0"

# --- Release ---
# Tag and push:  make release
# Dry run:        make release-dry
# Force bump:     make release BUMP=minor

.PHONY: release release-dry

release:
	@bash scripts/release.sh

release-dry:
	@bash scripts/release.sh --dry-run