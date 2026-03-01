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
4. **`internal/hosts/`** — YAML-based host registry with tag system. In-memory `map[tag][]Host` index for AND/OR/Search queries (wildcards via `path.Match`). Also manages `~/.ssh/config` entries and PuTTY sessions (Windows). `mobaimport.go` parses MobaXterm `.mxtsessions` files.
5. **`internal/ssh/`** — SSH/SCP command construction (`user@target@host@proxy -i key`), multi-pane session management (WezTerm, Tabby, Windows Terminal, tmux), MobaXterm tab launching, WinSCP session launching, and batch SCP operations.
6. **`internal/config/`** — YAML config with 15 settable keys. No company-specific defaults in code — all environment-specific values (URLs, hostnames) are set by the user via `config init`.
7. **`cmd/sogark/`** — Cobra command wiring. Each file maps to a command: `ssh.go`, `scp.go`, `login.go`, `keys.go`, `hosts.go`, `multi.go`, `moba.go`, `winscp.go`, `config.go`, `completion.go`.

## Commands

| Command | File | Description |
|---------|------|-------------|
| `sogark ssh` | `ssh.go` | SSH via PSMP with auto-auth |
| `sogark scp` | `scp.go` | SCP via PSMP, batch with #tag syntax |
| `sogark login` | `login.go` | SAML/MFA authentication only |
| `sogark keys` | `keys.go` | Key management + `keys clean` |
| `sogark config` | `config.go` | `init`, `set`, `show`, `wezterm` subcommands |
| `sogark hosts` | `hosts.go` | `add`, `list`, `remove`, `tag`, `search`, `import-moba` |
| `sogark multi` | `multi.go` | Multi-pane SSH: WezTerm/Tabby/WT/tmux backends |
| `sogark moba` | `moba.go` | MobaXterm multi-tab SSH (Windows) |
| `sogark winscp` | `winscp.go` | WinSCP SCP/SFTP sessions (Windows) |
| `sogark completion` | `completion.go` | Shell completion (bash/zsh/fish/powershell) |

## Config Fields (15 settable keys)

`username`, `pvwa_base_url`, `idp_url`, `proxy_host`, `key_dir`, `key_formats`, `default_target_user`, `default_scp_user`, `ssh_key_name`, `key_ttl_hours`, `saml_timeout_minutes`, `moba_path`, `moba_max_sessions`, `tabby_path`, `winscp_path`

Generic defaults only: `key_dir=~/.sogark/keys`, `key_formats=[OpenSSH,PEM,PPK]`, `key_ttl_hours=4`, `saml_timeout_minutes=5`, `moba_max_sessions=20`. All company-specific values are empty and set by user.

## Multi-pane backends (internal/ssh/multi.go)

| Backend | Function | Auto-detect |
|---------|----------|-------------|
| wezterm | `runMultiWezTerm()` | `$TERM_PROGRAM=WezTerm` |
| tabby | `RunTabby()` | `FindTabby()` in PATH/common dirs |
| wt | `runMultiWT()` | `wt.exe` in PATH |
| tmux | `runMultiTmux()` | `tmux` in PATH |

Additional launchers: `RunMoba()` (MobaXterm tabs), `RunWinSCP()` (WinSCP sessions).

## Key Conventions

- **User config**: `~/.sogark/config.yaml` (YAML via `gopkg.in/yaml.v3`)
- **Host registry**: `~/.sogark/hosts.yaml` with tag-based grouping
- **SSH key storage**: `~/.sogark/keys/` with `.key_timestamp` for TTL
- **SSH format**: `corporate_user@target_user@host@proxy_host -i key_path`
- **SCP target user**: `default_scp_user` → `default_target_user` fallback
- **Error messages and UI**: in Italian
- **Platform-specific code**: build tags (`putty_windows.go` / `putty_other.go`)
- **The `doLogin()` function** in `ssh.go` is the shared login flow used by ssh, scp, keys, multi, moba, and winscp commands
- **No company-specific defaults**: Sogei-specific values documented in `docs/user-guide.md` only
