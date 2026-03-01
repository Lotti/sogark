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

# --- Nexus publishing ---
# Override these via env or command line:
#   make publish VERSION=v1.0.0 NEXUS_USER=deploy NEXUS_PASS=secret
NEXUS_URL  ?= https://alm-repos.sogei.it
NEXUS_REPO ?= sogark-releases
NEXUS_USER ?=
NEXUS_PASS ?=
NEXUS_BASE  = $(NEXUS_URL)/repository/$(NEXUS_REPO)

.PHONY: publish

publish: build-all
ifndef NEXUS_USER
	$(error NEXUS_USER non impostato. Uso: make publish VERSION=v1.0.0 NEXUS_USER=user NEXUS_PASS=pass)
endif
ifndef NEXUS_PASS
	$(error NEXUS_PASS non impostato. Uso: make publish VERSION=v1.0.0 NEXUS_USER=user NEXUS_PASS=pass)
endif
	@echo "[*] Pubblicazione sogark $(VERSION) su $(NEXUS_BASE)..."
	@# Write version.txt
	@echo "$(VERSION)" > bin/version.txt
	@# Generate install scripts with baked-in URLs
	@sed -e 's|__NEXUS_URL__|$(NEXUS_URL)|g' -e 's|__NEXUS_REPO__|$(NEXUS_REPO)|g' \
		scripts/install.sh > bin/install.sh
	@sed -e 's|__NEXUS_URL__|$(NEXUS_URL)|g' -e 's|__NEXUS_REPO__|$(NEXUS_REPO)|g' \
		scripts/install.ps1 > bin/install.ps1
	@# Upload to versioned path
	@for f in $(BINARY_NAME)-darwin-arm64 $(BINARY_NAME)-darwin-amd64 \
	          $(BINARY_NAME)-linux-amd64 $(BINARY_NAME)-windows-amd64.exe \
	          version.txt install.sh install.ps1; do \
		echo "  -> $(VERSION)/$$f"; \
		curl -sf -u $(NEXUS_USER):$(NEXUS_PASS) --upload-file bin/$$f \
			"$(NEXUS_BASE)/$(VERSION)/$$f" || exit 1; \
	done
	@# Upload to latest/
	@for f in $(BINARY_NAME)-darwin-arm64 $(BINARY_NAME)-darwin-amd64 \
	          $(BINARY_NAME)-linux-amd64 $(BINARY_NAME)-windows-amd64.exe \
	          version.txt install.sh install.ps1; do \
		echo "  -> latest/$$f"; \
		curl -sf -u $(NEXUS_USER):$(NEXUS_PASS) --upload-file bin/$$f \
			"$(NEXUS_BASE)/latest/$$f" || exit 1; \
	done
	@echo "[✓] Pubblicazione completata: $(VERSION)"
