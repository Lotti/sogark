package ssh

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// MultiArgs holds parameters for a multi-pane session.
type MultiArgs struct {
	SessionName string
	Hosts       []HostTarget
	Sync        bool
	Backend     string // "auto", "tmux", "wt", "wezterm", "tabby"
	TabbyPath   string // optional override for Tabby executable
}

// HostTarget represents a single host for multi/exec commands.
type HostTarget struct {
	Name       string
	Address    string
	TargetUser string
}

// RunMulti opens a multi-pane session using the best available backend.
func RunMulti(args *MultiArgs, username, proxyHost, keyPath string) error {
	if len(args.Hosts) == 0 {
		return fmt.Errorf("nessun host specificato")
	}

	backend := args.Backend
	if backend == "" || backend == "auto" {
		backend = detectMultiBackend()
	}

	switch backend {
	case "wezterm":
		return runMultiWezTerm(args, username, proxyHost, keyPath)
	case "tabby":
		tabbyBin := args.TabbyPath
		if tabbyBin == "" {
			tabbyBin = FindTabby()
		}
		if tabbyBin == "" {
			return fmt.Errorf("Tabby non trovato. Usa 'sogark config set tabby_path /path/to/tabby'")
		}
		return RunTabby(args.Hosts, username, proxyHost, keyPath, tabbyBin)
	case "wt":
		return runMultiWT(args, username, proxyHost, keyPath)
	case "tmux":
		return runMultiTmux(args, username, proxyHost, keyPath)
	default:
		return fmt.Errorf("backend %q non supportato (usa 'wezterm', 'tabby', 'wt' o 'tmux')", backend)
	}
}

// detectMultiBackend selects the best available multi-pane backend.
func detectMultiBackend() string {
	// If inside WezTerm, prefer wezterm backend (supports sync input)
	if os.Getenv("TERM_PROGRAM") == "WezTerm" {
		return "wezterm"
	}
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("wt.exe"); err == nil {
			return "wt"
		}
		if FindTabby() != "" {
			return "tabby"
		}
	}
	if _, err := exec.LookPath("tmux"); err == nil {
		return "tmux"
	}
	if FindTabby() != "" {
		return "tabby"
	}
	if _, err := exec.LookPath("wt.exe"); err == nil {
		return "wt"
	}
	return "tmux" // will fail with helpful message
}

// runMultiTmux opens a tmux session with synchronized panes for each host.
func runMultiTmux(args *MultiArgs, username, proxyHost, keyPath string) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux non trovato. Installalo con:\n" +
			"  macOS:  brew install tmux\n" +
			"  Linux:  sudo apt install tmux")
	}

	sessionName := args.SessionName
	if sessionName == "" {
		sessionName = "sogark"
	}

	// Create the first pane with the first host
	first := args.Hosts[0]
	sshCmd := buildSSHCmd(username, first.TargetUser, first.Address, proxyHost, keyPath)

	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, sshCmd)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("errore creazione sessione tmux: %w", err)
	}

	// Add remaining hosts as split panes
	for _, h := range args.Hosts[1:] {
		sshCmd = buildSSHCmd(username, h.TargetUser, h.Address, proxyHost, keyPath)
		cmd = exec.Command("tmux", "split-window", "-t", sessionName, sshCmd)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("errore aggiunta pane per %s: %w", h.Name, err)
		}
		// Re-tile to keep panes evenly distributed
		exec.Command("tmux", "select-layout", "-t", sessionName, "tiled").Run()
	}

	// Enable synchronize-panes if requested
	if args.Sync {
		exec.Command("tmux", "set-window-option", "-t", sessionName, "synchronize-panes", "on").Run()
	}

	// Attach to the session
	attachCmd := exec.Command("tmux", "attach-session", "-t", sessionName)
	attachCmd.Stdin = os.Stdin
	attachCmd.Stdout = os.Stdout
	attachCmd.Stderr = os.Stderr
	return attachCmd.Run()
}

func buildSSHCmd(username, targetUser, host, proxyHost, keyPath string) string {
	user := fmt.Sprintf("%s@%s@%s@%s", username, targetUser, host, proxyHost)
	return fmt.Sprintf("ssh %s -i %s -o IdentitiesOnly=yes", user, keyPath)
}

// buildSogarkSSHArgs returns the sogark ssh command args for a host.
func buildSogarkSSHArgs(targetUser, host string) []string {
	target := targetUser + "@" + host
	return []string{"sogark", "ssh", target}
}

// runMultiWT opens Windows Terminal split panes for each host.
// Each pane runs "sogark ssh user@host" so it inherits key/config.
func runMultiWT(args *MultiArgs, username, proxyHost, keyPath string) error {
	wtExe, err := exec.LookPath("wt.exe")
	if err != nil {
		return fmt.Errorf("wt.exe non trovato. Installa Windows Terminal dal Microsoft Store")
	}

	sogarkExe, _ := os.Executable()

	// Build wt command with chained split-pane commands.
	// wt [new-tab cmd] ; sp -V cmd ; sp -H cmd ; ...
	// First host opens in the initial tab.
	first := args.Hosts[0]
	wtArgs := []string{
		"new-tab", "--title", first.Name, "--",
		sogarkExe, "ssh", first.TargetUser + "@" + first.Address,
	}

	// Remaining hosts as split panes, alternating vertical/horizontal
	for i, h := range args.Hosts[1:] {
		splitDir := "-V" // vertical split (side by side)
		if i%2 == 1 {
			splitDir = "-H" // horizontal split (top/bottom)
		}
		wtArgs = append(wtArgs, ";", "sp", splitDir, "--title", h.Name, "--",
			sogarkExe, "ssh", h.TargetUser+"@"+h.Address)
	}

	fmt.Printf("[+] Apertura Windows Terminal con %d pane...\n", len(args.Hosts))
	for _, h := range args.Hosts {
		fmt.Printf("    %s (%s@%s)\n", h.Name, h.TargetUser, h.Address)
	}
	if args.Sync {
		fmt.Println("[!] Windows Terminal non supporta input sincronizzato.")
		fmt.Println("    Per sync usa tmux (es. via WSL): sogark multi --backend tmux ...")
	}

	cmd := exec.Command(wtExe, wtArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runMultiWezTerm creates WezTerm split panes and broadcasts input to all.
// Uses wezterm cli split-pane + send-text for synchronized command input.
// Layout: SSH panes in a grid on top, broadcaster strip at bottom.
func runMultiWezTerm(args *MultiArgs, username, proxyHost, keyPath string) error {
	if os.Getenv("TERM_PROGRAM") != "WezTerm" {
		return fmt.Errorf("backend wezterm richiede di essere dentro WezTerm")
	}

	n := len(args.Hosts)
	if n > 8 {
		return fmt.Errorf("max 8 host per sessione WezTerm (hai %d). Dividi in batch più piccoli", n)
	}

	weztermBin, err := exec.LookPath("wezterm")
	if err != nil {
		return fmt.Errorf("wezterm CLI non trovato nel PATH")
	}

	sogarkExe, _ := os.Executable()

	// Step 1: split top 85% for first SSH pane, bottom 15% stays as broadcaster
	firstHost := args.Hosts[0]
	firstArgs := []string{"cli", "split-pane", "--top", "--percent", "85", "--",
		sogarkExe, "ssh", firstHost.TargetUser + "@" + firstHost.Address}
	out, err := exec.Command(weztermBin, firstArgs...).Output()
	if err != nil {
		return fmt.Errorf("errore split-pane per %s: %w", firstHost.Name, err)
	}
	firstPaneID := strings.TrimSpace(string(out))
	paneIDs := []string{firstPaneID}

	if n > 1 {
		// Determine grid: max 4 cols per row
		cols := n
		if cols > 4 {
			cols = (n + 1) / 2 // ceil(n/2) for top row
		}
		rows := (n + cols - 1) / cols

		if rows == 1 {
			// Single row: split firstPaneID into columns
			paneIDs = weztermSplitRow(weztermBin, sogarkExe, firstPaneID, args.Hosts, paneIDs)
		} else {
			// Two rows: split firstPaneID vertically first
			topRow := args.Hosts[:cols]
			bottomRow := args.Hosts[cols:]

			// Split first pane --bottom 50% to create second row
			bottomFirstArgs := []string{"cli", "split-pane", "--bottom", "--percent", "50",
				"--pane-id", firstPaneID, "--",
				sogarkExe, "ssh", bottomRow[0].TargetUser + "@" + bottomRow[0].Address}
			out, err := exec.Command(weztermBin, bottomFirstArgs...).Output()
			if err != nil {
				return fmt.Errorf("errore split-pane riga 2: %w", err)
			}
			bottomFirstPaneID := strings.TrimSpace(string(out))

			// Split top row into columns
			paneIDs = weztermSplitRow(weztermBin, sogarkExe, firstPaneID, topRow, paneIDs)
			// Split bottom row into columns
			bottomPaneIDs := []string{bottomFirstPaneID}
			bottomPaneIDs = weztermSplitRow(weztermBin, sogarkExe, bottomFirstPaneID, bottomRow, bottomPaneIDs)
			paneIDs = append(paneIDs, bottomPaneIDs[1:]...)
		}
	}

	// Focus the broadcaster pane (the original pane we're running in)
	broadcasterPaneID := os.Getenv("WEZTERM_PANE")
	if broadcasterPaneID != "" {
		exec.Command(weztermBin, "cli", "activate-pane", "--pane-id", broadcasterPaneID).Run()
	}

	fmt.Printf("[+] WezTerm: %d pane SSH aperti\n", n)
	for _, h := range args.Hosts {
		fmt.Printf("    %s (%s@%s)\n", h.Name, h.TargetUser, h.Address)
	}

	if !args.Sync {
		fmt.Println("[i] Input non sincronizzato (--no-sync)")
		return nil
	}

	return weztermBroadcastLoop(weztermBin, paneIDs)
}

// weztermSplitRow splits a pane into N equal columns for the hosts.
// Returns updated paneIDs slice with new pane IDs appended.
func weztermSplitRow(weztermBin, sogarkExe, parentPaneID string, hosts []HostTarget, paneIDs []string) []string {
	// hosts[0] is already in parentPaneID, split for hosts[1:]
	lastPaneID := parentPaneID
	remaining := len(hosts)

	for i := 1; i < len(hosts); i++ {
		// Calculate percent for equal distribution
		// remaining-1 slots left in remaining space, new pane gets (remaining-i)/(remaining-i+1)
		pct := 100 * (remaining - i) / (remaining - i + 1)
		if pct < 20 {
			pct = 50
		}

		splitArgs := []string{"cli", "split-pane", "--right", "--percent", fmt.Sprintf("%d", pct),
			"--pane-id", lastPaneID, "--",
			sogarkExe, "ssh", hosts[i].TargetUser + "@" + hosts[i].Address}
		out, err := exec.Command(weztermBin, splitArgs...).Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[!] Errore split-pane per %s: %v\n", hosts[i].Name, err)
			continue
		}
		newPaneID := strings.TrimSpace(string(out))
		paneIDs = append(paneIDs, newPaneID)
		lastPaneID = newPaneID
	}
	return paneIDs
}

// weztermBroadcastLoop reads lines from stdin and sends them to all panes.
// Exits when Ctrl+D is pressed or all SSH panes are closed.
func weztermBroadcastLoop(weztermBin string, paneIDs []string) error {
	fmt.Println("[+] Broadcast attivo. Digita comandi (Ctrl+D per uscire):")
	fmt.Println()

	// Start a goroutine that monitors pane liveness
	done := make(chan struct{})
	go func() {
		for {
			time.Sleep(3 * time.Second)
			if !weztermAnyPaneAlive(weztermBin, paneIDs) {
				close(done)
				return
			}
		}
	}()

	lineCh := make(chan string)
	eofCh := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
		close(eofCh)
	}()

	for {
		fmt.Print("[sogark] > ")
		select {
		case line, ok := <-lineCh:
			if !ok {
				fmt.Println("\n[+] Broadcast terminato.")
				return nil
			}
			if line == "" {
				weztermSendText(weztermBin, paneIDs, "\n")
			} else {
				weztermSendText(weztermBin, paneIDs, line+"\n")
			}
		case <-eofCh:
			fmt.Println("\n[+] Broadcast terminato.")
			return nil
		case <-done:
			fmt.Println("\n[+] Tutti i pane SSH chiusi. Broadcast terminato.")
			return nil
		}
	}
}

// weztermSendText sends text to all specified WezTerm panes.
func weztermSendText(weztermBin string, paneIDs []string, text string) {
	for _, pid := range paneIDs {
		cmd := exec.Command(weztermBin, "cli", "send-text", "--pane-id", pid, "--no-paste")
		cmd.Stdin = strings.NewReader(text)
		cmd.Run()
	}
}

// weztermAnyPaneAlive checks if any of the given pane IDs still exist in WezTerm.
func weztermAnyPaneAlive(weztermBin string, paneIDs []string) bool {
	out, err := exec.Command(weztermBin, "cli", "list", "--format", "json").Output()
	if err != nil {
		return false
	}
	var panes []struct {
		PaneID int `json:"pane_id"`
	}
	if err := json.Unmarshal(out, &panes); err != nil {
		return false
	}

	alive := make(map[string]bool)
	for _, p := range panes {
		alive[fmt.Sprintf("%d", p.PaneID)] = true
	}
	for _, pid := range paneIDs {
		if alive[pid] {
			return true
		}
	}
	return false
}

// RunMoba launches MobaXterm with one tab per host.
// maxSessions limits the number of tabs opened (0 = use default of 20).
func RunMoba(hosts []HostTarget, username, proxyHost, keyPath, mobaPath string, maxSessions int) error {
	if len(hosts) == 0 {
		return fmt.Errorf("nessun host specificato")
	}

	if mobaPath == "" {
		return fmt.Errorf("MobaXterm non trovato")
	}

	if maxSessions <= 0 {
		maxSessions = 20
	}
	if len(hosts) > maxSessions {
		fmt.Fprintf(os.Stderr, "[!] Troppi host (%d), limite sessioni MobaXterm: %d. Verranno aperte solo le prime %d sessioni.\n",
			len(hosts), maxSessions, maxSessions)
		hosts = hosts[:maxSessions]
	}

	fmt.Printf("[+] Apertura MobaXterm con %d tab...\n", len(hosts))
	for i, h := range hosts {
		// MobaXterm's embedded SSH interprets backslashes as escape chars,
		// so convert Windows paths to forward slashes
		mobaKeyPath := strings.ReplaceAll(keyPath, "\\", "/")
		sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes %s@%s@%s@%s",
			mobaKeyPath, username, h.TargetUser, h.Address, proxyHost)
		fmt.Printf("    %s (%s@%s)\n", h.Name, h.TargetUser, h.Address)

		cmd := exec.Command(mobaPath, "-newtab", sshCmd)
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Errore apertura tab per %s: %v\n", h.Name, err)
		}
		// MobaXterm needs time between tab launches
		if i < len(hosts)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	fmt.Println("\n[i] Per attivare MultiExec: click destro su un tab → Multi-execution")
	return nil
}

// FindMobaXterm searches for MobaXterm in common locations.
func FindMobaXterm() string {
	return findMobaXterm()
}

// findMobaXterm searches for MobaXterm in common locations.
func findMobaXterm() string {
	// Check PATH first
	if p, err := exec.LookPath("MobaXterm.exe"); err == nil {
		return p
	}
	if p, err := exec.LookPath("MobaXterm_Personal.exe"); err == nil {
		return p
	}

	// Common install locations
	candidates := []string{
		os.Getenv("ProgramFiles") + "\\Mobatek\\MobaXterm\\MobaXterm.exe",
		os.Getenv("ProgramFiles(x86)") + "\\Mobatek\\MobaXterm\\MobaXterm.exe",
		os.Getenv("LOCALAPPDATA") + "\\Programs\\MobaXterm\\MobaXterm.exe",
	}
	// Also check home directory for portable version
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			home+"\\MobaXterm\\MobaXterm_Personal.exe",
			home+"\\Desktop\\MobaXterm_Personal.exe",
		)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// FindTabby searches for Tabby terminal in common locations.
func FindTabby() string {
	// PATH first
	for _, name := range []string{"tabby", "tabby.exe"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}

	candidates := []string{
		os.Getenv("LOCALAPPDATA") + "\\Programs\\Tabby\\tabby.exe",
		os.Getenv("ProgramFiles") + "\\Tabby\\tabby.exe",
		"/Applications/Tabby.app/Contents/MacOS/Tabby",
		"/usr/local/bin/tabby",
		"/usr/bin/tabby",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			home+"/Applications/Tabby.app/Contents/MacOS/Tabby",
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// RunTabby opens Tabby terminal with one SSH tab per host.
func RunTabby(hosts []HostTarget, username, proxyHost, keyPath, tabbyPath string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("nessun host specificato")
	}
	if tabbyPath == "" {
		return fmt.Errorf("Tabby non trovato")
	}

	fmt.Printf("[+] Apertura Tabby con %d tab...\n", len(hosts))
	for i, h := range hosts {
		sshUser := fmt.Sprintf("%s@%s@%s@%s", username, h.TargetUser, h.Address, proxyHost)
		// Tabby supports opening SSH via URL scheme: tabby open ssh://user@host
		sshURL := fmt.Sprintf("ssh://%s", sshUser)
		fmt.Printf("    %s (%s@%s)\n", h.Name, h.TargetUser, h.Address)

		cmd := exec.Command(tabbyPath, "open", sshURL)
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Errore apertura tab per %s: %v\n", h.Name, err)
		}
		if i < len(hosts)-1 {
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}

// FindWinSCP searches for WinSCP in common Windows locations.
func FindWinSCP() string {
	for _, name := range []string{"WinSCP.exe", "winscp.exe"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}

	candidates := []string{
		os.Getenv("ProgramFiles") + "\\WinSCP\\WinSCP.exe",
		os.Getenv("ProgramFiles(x86)") + "\\WinSCP\\WinSCP.exe",
		os.Getenv("LOCALAPPDATA") + "\\Programs\\WinSCP\\WinSCP.exe",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			home+"\\WinSCP\\WinSCP.exe",
			home+"\\Desktop\\WinSCP.exe",
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// RunWinSCP opens WinSCP with one SCP session per host using the PSMP proxy format.
// keyPath should be a .ppk file (PuTTY format) for best WinSCP compatibility.
func RunWinSCP(hosts []HostTarget, username, proxyHost, keyPath, winscpPath string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("nessun host specificato")
	}
	if winscpPath == "" {
		return fmt.Errorf("WinSCP non trovato")
	}

	fmt.Printf("[+] Apertura WinSCP con %d sessioni...\n", len(hosts))
	for i, h := range hosts {
		// WinSCP connection string: scp://user@host/ with private key
		scpUser := fmt.Sprintf("%s@%s@%s@%s", username, h.TargetUser, h.Address, proxyHost)
		connStr := fmt.Sprintf("scp://%s/", scpUser)

		args := []string{connStr}
		if keyPath != "" {
			winKeyPath := strings.ReplaceAll(keyPath, "/", "\\")
			// Prefer .ppk for WinSCP; fall back to the given path
			ppkPath := strings.TrimSuffix(winKeyPath, ".pem")
			ppkPath = strings.TrimSuffix(ppkPath, "")
			if !strings.HasSuffix(ppkPath, ".ppk") {
				ppkPath += ".ppk"
			}
			if _, err := os.Stat(ppkPath); err == nil {
				args = append([]string{"/privatekey=" + ppkPath}, args...)
			} else {
				args = append([]string{"/privatekey=" + winKeyPath}, args...)
			}
		}

		fmt.Printf("    %s (%s@%s)\n", h.Name, h.TargetUser, h.Address)
		cmd := exec.Command(winscpPath, args...)
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Errore apertura sessione per %s: %v\n", h.Name, err)
		}
		if i < len(hosts)-1 {
			time.Sleep(2 * time.Second)
		}
	}
	return nil
}
