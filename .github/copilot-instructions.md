# Copilot Instructions

## Build & Test

```bash
make build          # compila bin/sogark
make test           # go test ./...
make build-all      # cross-compile (darwin/linux/windows)
go test ./internal/keys/... -run TestParse  # singolo test
```

## Architecture

`sogark` is a Go CLI (cobra) for CyberArk PSMP SSH access via SAML/MFA authentication. The flow is:

1. **`internal/auth/saml.go`** — Opens Chrome via `go-rod/rod`, navigates to the IDP URL, waits for the user to authenticate (including MFA), then extracts the `SAMLResponse` hidden input from the DOM.
2. **`internal/auth/cyberark.go`** — CyberArk PVWA REST API client: POSTs the SAMLResponse to `/API/auth/SAML/Logon/` for a session token, then POSTs to `/API/Users/Secret/SSHKeys/Cache` to fetch SSH keys.
3. **`internal/keys/`** — Parses key blocks (PEM, OpenSSH, PPK) from the raw API response via regex, saves them with `0600` permissions, and manages a `.key_timestamp` file for TTL validation.
4. **`internal/hosts/`** — YAML-based host registry with tag system. In-memory `map[tag][]Host` index for AND/OR queries. Also manages `~/.ssh/config` entries and PuTTY sessions (Windows).
5. **`internal/ssh/`** — SSH command construction (`user@target@host@proxy -i key`), tmux multi-session management, and parallel exec with goroutines.
6. **`cmd/sogark/`** — Cobra command wiring. Each file maps to a command: `ssh.go`, `scp.go`, `login.go`, `keys.go`, `hosts.go`, `multi.go`, `exec.go`, `config.go`.

## Key Conventions

- **User config**: `~/.sogark/config.yaml` (YAML via `gopkg.in/yaml.v3`)
- **Host registry**: `~/.sogark/hosts.yaml` with tag-based grouping
- **SSH key storage**: `~/.sogark/keys/` with `.key_timestamp` for TTL
- **SSH format**: `corporate_user@target_user@host@proxy_host -i key_path`
- **Error messages and UI**: in Italian
- **Platform-specific code**: build tags (`putty_windows.go` / `putty_other.go`)
- **The `doLogin()` function** in `ssh.go` is the shared login flow used by ssh, scp, keys, multi, and exec commands
