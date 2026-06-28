// Package messages centralises all user-facing strings so that translations
// only require editing this single file.
package messages

// ── Shared ────────────────────────────────────────────────────────────────────

const (
	KeyValid         = "[+] Valid key (expires in %s)\n"
	KeyValidFull     = "[+] Valid key (expires in %dh %dm)\n"
	KeyExpired       = "[!] Key expired or missing, starting authentication..."
	KeysSaved        = "[+] Keys saved:"
	KeysExpiry       = "  Expiry: in %dh\n"
	DownloadingKeys  = "[*] Downloading keys from CyberArk..."
	AuthComplete     = "[+] Authentication complete"
	FlagRequiresValue = "flag %s requires a value"
)

// ── SSH command ───────────────────────────────────────────────────────────────

const (
	SSHShort = "SSH connection via PSMP with automatic authentication"
	SSHLong  = `Full flow: key check -> SAML/MFA authentication if needed -> SSH connection.

If the host matches a name registered in hosts.yaml, resolves its address and user.
All standard ssh flags are supported directly.

Sogark flags (--dry-run, --force-login, -u, --key-format) must precede the host.`
	SSHErrNoHost = "specify the host\nExample: sogark ssh 10.1.2.3"
)

// ── SCP command ───────────────────────────────────────────────────────────────

const (
	SCPShort = "File transfer via SCP through PSMP"
	SCPLong  = `Transparent scp wrapper: sogark injects the SSH key (-i) and rewrites remote paths to PSMP format.

Remote paths (host:path or user@host:path) are automatically rewritten:
  host:/path  →  corp@target@host@psmp:/path

Sogark flags (--dry-run, --force-login, -u, --key-format, --tag, --any-tag) must precede scp flags.
All other flags are passed directly to scp.

If the SSH key has expired, authentication is performed automatically.

Batch mode with --tag/--any-tag: sends files to all hosts with the tag.
Use ":/path" to specify the remote path on each host.`
	SCPExample = `  # Upload file
  sogark scp file.txt 10.1.2.3:/tmp/

  # Upload with inline tag
  sogark scp file.txt #webservers:/tmp/
  sogark scp file.txt oper1@#web#prod:/tmp/

  # Download with inline tag (creates subfolders per host)
  sogark scp #webservers:/etc/hosts ./configs/

  # Upload to all hosts with --tag flag
  sogark scp --tag webservers file.txt :/tmp/

  # Upload directory to multiple tags (OR)
  sogark scp --any-tag web,app -r ./deploy :/opt/app/

  # Upload directory
  sogark scp -r ./mydir 10.1.2.3:/opt/

  # Download file
  sogark scp 10.1.2.3:/etc/hosts ./

  # With specific target user
  sogark scp file.txt admin@10.1.2.3:/tmp/

  # With native scp flags (compression, verbose, port)
  sogark scp -C -v -P 2222 file.txt 10.1.2.3:/tmp/

  # Dry run (show command without executing)
  sogark scp --dry-run file.txt 10.1.2.3:/tmp/`
	SCPErrNoArgs           = "specify source and target\nExample: sogark scp file.txt host:/tmp/"
	SCPFlagTagRequired     = "flag --tag requires a value"
	SCPFlagAnyTagRequired  = "flag --any-tag requires a value"
)

// ── Login command ─────────────────────────────────────────────────────────────

const (
	LoginShort      = "SAML/MFA authentication and SSH key download"
	LoginLong       = "Opens the browser for SAML/MFA authentication, downloads SSH keys from CyberArk and saves them to disk."
	LoginFlagUser   = "override corporate username"
	LoginFlagFormat = "key formats (openssh,pem,ppk)"
)

// ── Keys command ──────────────────────────────────────────────────────────────

const (
	KeysCmdShort        = "Download or manage SSH keys"
	KeysCmdLong         = "Downloads SSH keys from CyberArk and saves them to the specified directory."
	KeysCleanShort      = "Delete downloaded SSH keys"
	KeysCleanPrompt     = "Delete keys in %s? [y/N] "
	KeysCleanCancelled  = "Operation cancelled."
	KeysCleanNoFiles    = "No files to remove."
	KeysCleanRemoved    = "[+] Removed: %s\n"
	KeysFlagDir         = "output directory (default: from config)"
	KeysFlagFormat      = "formats: openssh,pem,ppk"
	KeysFlagForceLogin  = "force re-authentication"
	KeysCleanFlagDir    = "directory to clean (default: from config)"
	KeysCleanFlagYes    = "skip confirmation"
)

// ── Multi command ─────────────────────────────────────────────────────────────

const (
	MultiShort = "Parallel SSH sessions with synchronized panes"
	MultiLong  = `Opens a multi-pane session with one pane per host.
Backend auto-detect: WezTerm (with input sync) > Windows Terminal > tmux.
If running inside WezTerm, uses the wezterm backend with automatic broadcast.
Use --backend to force a specific backend.`
	MultiErrNoHostOrTag  = "specify host or tag (--tag / --any-tag)"
	MultiErrNoHostsFound = "no hosts found"
	MultiSelectedHosts   = "Selected hosts: %s\n"
	MultiFlagTag         = "filter by tag (AND)"
	MultiFlagAnyTag      = "filter by tag (OR)"
	MultiFlagNoSync      = "do not synchronize input between panes (tmux only)"
	MultiFlagBackend     = "multi-pane backend: auto, wezterm, tabby, wt, tmux"
)

// ── Moba command ──────────────────────────────────────────────────────────────

const (
	MobaShort           = "Open SSH sessions in MobaXterm"
	MobaLong            = `Opens MobaXterm with one SSH tab per selected host.
After opening, activate MultiExec to send commands to all tabs.`
	MobaNotFound        = "[!] MobaXterm not found."
	MobaEnterPath       = "    Enter the path to MobaXterm.exe: "
	MobaFileNotFound    = "[!] File not found: %s\n"
	MobaErrSavingConfig = "[!] Error saving config: %v\n"
	MobaPathSaved       = "[+] Path saved to configuration: %s\n"
	MobaFlagTag         = "filter by tag (AND)"
	MobaFlagAnyTag      = "filter by tag (OR)"
	MobaFlagPath        = "path to MobaXterm.exe"
)

// ── WinSCP command ────────────────────────────────────────────────────────────

const (
	WinSCPShort    = "Open SCP/SFTP sessions in WinSCP (Windows)"
	WinSCPLong     = `Opens WinSCP with one session per host, using the CyberArk PSMP format.
Supports auto-detection of WinSCP in standard directories.
Use --winscp-path to manually specify the path.`
	WinSCPNotFound = "WinSCP not found.\n" +
		"Set the path with:\n" +
		"  sogark config set winscp_path \"C:\\WinSCP\\WinSCP.exe\"\n" +
		"or use --winscp-path"
	WinSCPFlagTag    = "filter by tag (AND)"
	WinSCPFlagAnyTag = "filter by tag (OR)"
	WinSCPFlagPath   = "manual path to WinSCP.exe"
)

// ── Config command ────────────────────────────────────────────────────────────

const (
	ConfigShort            = "Manage sogark configuration"
	ConfigInitShort        = "Interactive wizard for initial configuration"
	ConfigInitTitle        = "sogark configuration"
	ConfigInitUsername     = "Corporate username"
	ConfigInitSSHKeyName   = "SSH key name"
	ConfigInitKeyDir       = "Key directory"
	ConfigInitSSHUser      = "Default SSH target user"
	ConfigInitSCPUser      = "Default SCP target user (empty = same as SSH)"
	ConfigInitKeyFormats   = "Key formats"
	ConfigSavedAt          = "\n[+] Configuration saved to %s\n"
	ConfigSetShort         = "Set a configuration parameter"
	ConfigShowShort        = "Show current configuration"
	ConfigWeztermShort     = "Generate WezTerm configuration file for VM"
	ConfigWeztermLong      = `Generates ~/.wezterm.lua with software rendering (for VMs with limited GPU)
and clipboard support (Ctrl+Shift+C/V).
If the file already exists, prints instructions for manual configuration.`
	ConfigWeztermFileExists = "[i] File %s already exists.\n"
	ConfigWeztermAddLines   = "    Add these lines manually to your configuration:"
	ConfigWeztermRenderComment = "  -- Rendering for VM (if OpenGL doesn't work)"
	ConfigWeztermOrComment     = "  -- or: front_end = \"Software\","
	ConfigWeztermSaved         = "[+] WezTerm configuration saved to %s\n"
	ConfigWeztermEnabled       = "    Software rendering + clipboard enabled."
	ConfigErrHomeDir           = "cannot determine home directory: %w"
	ConfigErrWriteLua          = "error writing %s: %w"
)

// ── Main / root command ───────────────────────────────────────────────────────

const (
	RootShort       = "CyberArk PSMP CLI — SAML/MFA authentication and SSH session management"
	RootInterrupted = "\n[!] Operation interrupted"
	RootFlagVerbose = "detailed output for debugging"
)

// ── Update command ────────────────────────────────────────────────────────────

const (
	UpdateShort = "Update sogark to the latest version"
	UpdateLong  = `Checks the latest version available on GitHub Releases
and updates the current binary if necessary.

Requires update_repo to be configured:
  sogark config set update_repo user/sogark`
	UpdateExample = `  sogark update              # update to the latest version
  sogark update --check      # check without updating
  sogark update --version v1.2.0  # install specific version
  sogark update --force      # force re-download even if up to date`
	UpdateErrNotConfigured = "update_repo not configured.\nRun:\n" +
		"  sogark config set update_repo user/sogark"
	UpdateCheckingVersion  = "[*] Checking latest available version..."
	UpdateErrFetchVersion  = "error fetching version: %w"
	UpdateCurrentVersion   = "[*] Current version: %s\n"
	UpdateAvailableVersion = "[*] Available version: %s\n"
	UpdateAlreadyUpToDate  = "[✓] Already up to date."
	UpdateAvailable        = "[!] Update available. Run 'sogark update' to update."
	UpdateErrExecPath      = "cannot determine executable path: %w"
	UpdateErrSymlink       = "cannot resolve symlink: %w"
	UpdateErrDownload      = "download error: %w"
	UpdateErrChmod         = "chmod error: %w"
	UpdateErrReplace       = "error replacing binary: %w"
	UpdateSuccess          = "[✓] Updated to %s\n"
	UpdateFlagVersion      = "specific version to install (e.g. v1.2.0)"
	UpdateFlagForce        = "force download even if version matches"
	UpdateFlagCheck        = "check without updating"
	UpdateHTTPErrVersion   = "HTTP %d from %s"
	UpdateHTTPErr          = "HTTP %d from %s"
)

// ── Hosts command ─────────────────────────────────────────────────────────────

const (
	HostsShort              = "Manage machine registry with tags"
	HostsAddShort           = "Register a host with optional tags"
	HostsAddSSHConfigErr    = "[!] ~/.ssh/config update failed: %v\n"
	HostsAddPuTTYErr        = "[!] PuTTY session: %v\n"
	HostsAddPuTTYSuccess    = "[+] PuTTY session created: %s\n"
	HostsAdded              = "[+] Host added: %s (%s)\n"
	HostsAddFlagUser        = "target user (default: from config)"
	HostsAddFlagTags        = "comma-separated tags"
	HostsAddFlagPutty       = "also create PuTTY session (Windows only)"
	HostsListShort          = "List registered hosts (filter by tag)"
	HostsListNoneFound      = "No hosts found."
	HostsListCount          = "\n%d hosts\n"
	HostsListFlagTag        = "filter by tag (AND: all tags must match)"
	HostsListFlagAnyTag     = "filter by tag (OR: at least one tag)"
	HostsRemoveShort        = "Remove a host from the registry"
	HostsRemoved            = "[+] Host removed: %s\n"
	HostsTagShort           = "Manage tags for a host"
	HostsTagFlagAdd         = "tags to add"
	HostsTagFlagRemove      = "tags to remove"
	HostsImportMobaShort    = "Import SSH sessions from a MobaXterm export"
	HostsImportMobaLong     = `Reads a .mxtsessions file exported from MobaXterm and imports SSH sessions
into the sogark registry. MobaXterm folders are converted to tags.`
	HostsImportMobaNoSessions  = "[i] No SSH sessions found in the file."
	HostsImportMobaPreview     = "[i] Preview: %d SSH sessions found\n"
	HostsImportMobaUserIgnored = "(ignored)"
	HostsImportMobaUserDefault = "(default)"
	HostsImportMobaErrSave     = "error saving registry: %w"
	HostsImportMobaSuccess     = "[+] Imported %d hosts from MobaXterm\n"
	HostsImportMobaFlagTag     = "extra tag to apply to all imported hosts"
	HostsImportMobaFlagDryRun  = "show preview without importing"
	HostsImportMobaFlagNoUser  = "ignore the user from MobaXterm sessions (use default_ssh_user from config)"
	HostsSearchShort           = "Search hosts in the registry by name, IP or tag"
	HostsSearchLong            = `Searches hosts in the registry. Supports wildcards (* and ?) for name and IP.
Criteria are combined with AND.
With --add-tag and/or --remove-tag modifies tags on the found hosts.`
	HostsSearchNoneFound   = "[i] No hosts found."
	HostsSearchTagsUpdated = "[+] Tags updated on %d hosts\n"
	HostsSearchCount       = "\n%d hosts found\n"
	HostsSearchErrSave     = "error saving registry: %w"
	HostsSearchFlagName    = "filter by name (supports wildcards * and ?)"
	HostsSearchFlagIP      = "filter by IP address (supports wildcards * and ?)"
	HostsSearchFlagTag     = "filter by tag (AND, comma-separated)"
	HostsSearchFlagAddTag  = "add tags to found hosts"
	HostsSearchFlagRemoveTag = "remove tags from found hosts"
)

// ── internal/auth ─────────────────────────────────────────────────────────────

const (
	AuthLogonFailed       = "logon failed: %w"
	AuthLogonReadErr      = "error reading logon response: %w"
	AuthLogonHTTPFailed   = "logon failed (HTTP %d): %s"
	AuthTokenNotReceived  = "session token not received"
	AuthNotAuthenticated  = "not authenticated: run login first"
	AuthSerializeErr      = "error serializing request: %w"
	AuthCreateRequestErr  = "error creating request: %w"
	AuthKeyFetchFailed    = "key fetch failed: %w"
	AuthReadKeysErr       = "error reading keys response: %w"
	AuthKeyFetchHTTPFailed = "key fetch failed (HTTP %d): %s"

	// saml_other.go (non-Windows)
	AuthBrowserNotFound    = "Chromium-based browser not found (Edge, Chrome, Chromium).\n" +
		"Install Edge or Chrome:\n" +
		"  macOS:  brew install --cask microsoft-edge\n" +
		"  Linux:  sudo apt install microsoft-edge-stable"
	AuthBrowserStartErr    = "error starting browser: %w"
	AuthBrowserConnectErr  = "error connecting to browser: %w"
	AuthBrowserPageErr     = "error opening browser page: %w"
	AuthSAMLScriptErr      = "error injecting SAML interception script: %w"
	AuthNavigateErr        = "error navigating to IDP: %w"
	AuthBrowserOpening     = "[*] Opening browser for SAML/MFA login..."
	AuthCompleteInBrowser  = "   Complete authentication in the browser."
	AuthSAMLTimeout        = "timeout: SAMLResponse not received (did you complete the login?)"

	// saml_windows.go
	AuthPSNotFound         = "powershell.exe not found.\nWindows PowerShell is required for SAML authentication."
	AuthWindowOpening      = "[*] Opening SAML/MFA login window..."
	AuthCompleteInWindow   = "   Complete authentication in the window."
	AuthSAMLFailed         = "SAML authentication failed: %s"
	AuthSAMLFailedW        = "SAML authentication failed: %w"
	AuthSAMLEmpty          = "empty SAMLResponse: login not completed or window closed"
)

// ── internal/config ───────────────────────────────────────────────────────────

const (
	CfgErrHomeDir      = "cannot determine home directory: %w"
	CfgNotFound        = "configuration not found: run 'sogark config init'"
	CfgReadErr         = "error reading config: %w"
	CfgParseErr        = "error parsing config: %w"
	CfgMkdirErr        = "error creating directory %s: %w"
	CfgSerializeErr    = "error serializing config: %w"
	CfgWriteErr        = "error writing config: %w"
	CfgKeyTTLHoursErr  = "key_ttl_hours must be a positive integer"
	CfgSAMLTimeoutErr  = "saml_timeout_minutes must be a positive integer"
	CfgMobaMaxErr      = "moba_max_sessions must be a positive integer"
	CfgInvalidBackend  = "invalid backend: %q (valid values: auto, wezterm, tabby, wt, tmux)"
	CfgUnknownKey      = "unknown key: %q\nValid keys: %s"
)

// ── internal/keys ─────────────────────────────────────────────────────────────

const (
	KeysMkdirErr     = "error creating directory %s: %w"
	KeysRemoveErr    = "error removing %s: %w"
	KeysWriteErr     = "error writing %s: %w"
	KeysReadTSErr    = "error reading timestamp: %w"
	KeysNoBlockFound = "no key block found in the response"
)

// ── internal/hosts ────────────────────────────────────────────────────────────

const (
	RegReadErr      = "error reading %s: %w"
	RegParseErr     = "error parsing %s: %w"
	RegNotFound     = "host %q not found"
	SSHConfigMkdir  = "error creating .ssh directory: %w"
	MobaOpenFileErr = "error opening file: %w"
)

// ── internal/ssh ──────────────────────────────────────────────────────────────

const (
	SSHNoHosts            = "no hosts specified"
	SSHTabbyNotFound      = "Tabby not found. Use 'sogark config set tabby_path /path/to/tabby'"
	SSHBackendNotSupported = "backend %q not supported (use 'wezterm', 'tabby', 'wt' or 'tmux')"
	SSHTmuxNotFound       = "tmux not found. Install it with:\n" +
		"  macOS:  brew install tmux\n" +
		"  Linux:  sudo apt install tmux"
	SSHTmuxCreateErr      = "error creating tmux session: %w"
	SSHTmuxAddPaneErr     = "error adding pane for %s: %w"
	SSHWTNotFound         = "wt.exe not found. Install Windows Terminal from the Microsoft Store"
	SSHWTOpening          = "[+] Opening Windows Terminal with %d panes...\n"
	SSHWTNoSync           = "[!] Windows Terminal does not support synchronized input."
	SSHWTSyncHint         = "    For sync use tmux (e.g. via WSL): sogark multi --backend tmux ..."
	SSHWeztermRequires    = "wezterm backend requires running inside WezTerm"
	SSHWeztermMaxHosts    = "max 8 hosts per WezTerm session (you have %d). Split into smaller batches"
	SSHWeztermCLINotFound = "wezterm CLI not found in PATH"
	SSHWeztermSplitErr    = "error splitting pane for %s: %w"
	SSHWeztermSplitRow2Err = "error splitting pane row 2: %w"
	SSHWeztermSplitErrFmt = "[!] Error splitting pane for %s: %v\n"
	SSHWeztermOpened      = "[+] WezTerm: %d SSH panes opened\n"
	SSHWeztermNoSync      = "[i] Input not synchronized (--no-sync)"
	SSHBroadcastActive    = "[+] Broadcast active. Type commands (Ctrl+D to exit):"
	SSHBroadcastEnded     = "\n[+] Broadcast ended."
	SSHAllPanesClosed     = "\n[+] All SSH panes closed. Broadcast ended."
	SSHMobaNotFound       = "MobaXterm not found"
	SSHMobaTooManyHosts   = "[!] Too many hosts (%d), MobaXterm session limit: %d. Only the first %d sessions will be opened.\n"
	SSHMobaNotRunning     = "[*] MobaXterm not running, starting..."
	SSHMobaStartErr       = "error starting MobaXterm: %w"
	SSHMobaWaiting        = "[*] Waiting for MobaXterm to initialize (10s)..."
	SSHMobaOpening        = "[+] Opening MobaXterm with %d tabs...\n"
	SSHMobaTabErr         = "[!] Error opening tab for %s: %v\n"
	SSHMobaMultiExec      = "\n[i] To enable MultiExec: right-click a tab → Multi-execution"
	SSHTabbyNotFoundSimple = "Tabby not found"
	SSHTabbyOpening       = "[+] Opening Tabby with %d tabs...\n"
	SSHTabbyTabErr        = "[!] Error opening tab for %s: %v\n"
	SSHWinSCPNotFound     = "WinSCP not found"
	SSHWinSCPOpening      = "[+] Opening WinSCP with %d sessions...\n"
	SSHWinSCPSessionErr   = "[!] Error opening session for %s: %v\n"

	PuTTYCreateSessionErr = "error creating PuTTY session: %w"
	PuTTYSetValueErr      = "error setting PuTTY value %s: %w"
	PuTTYDeleteSessionErr = "error deleting PuTTY session: %w"
)
