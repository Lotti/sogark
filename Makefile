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
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/sogark

# --- svu versioning ---
# svu is a tiny Go binary for semantic versioning from conventional commits.
# Install: go install github.com/caarlos0/svu/v3@latest

.PHONY: version

version:
	@svu current 2>/dev/null || echo "dev"

next-version:
	@svu next 2>/dev/null || echo "v0.1.0"

# --- Release (local, for CI) ---
# Creates release assets in bin/ for manual upload.
.PHONY: release-assets

release-assets: build-all
	@echo "[*] Generating release assets for $(VERSION)..."
	@echo "$(VERSION)" > bin/version.txt
	@# Generate install scripts with baked-in update_repo
	@sed 's|__UPDATE_REPO__|$(UPDATE_REPO)|g' scripts/install.sh > bin/install.sh
	@sed 's|__UPDATE_REPO__|$(UPDATE_REPO)|g' scripts/install.ps1 > bin/install.ps1
	@ls -lh bin/
	@echo "[✓] Assets ready in bin/"