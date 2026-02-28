package ssh

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// MultiArgs holds parameters for a multi-pane session.
type MultiArgs struct {
	SessionName string
	Hosts       []HostTarget
	Sync        bool
	Backend     string // "auto", "tmux", "wt", "wezterm"
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
	case "wt":
		return runMultiWT(args, username, proxyHost, keyPath)
	case "tmux":
		return runMultiTmux(args, username, proxyHost, keyPath)
	default:
		return fmt.Errorf("backend %q non supportato (usa 'wezterm', 'wt' o 'tmux')", backend)
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
	}
	if _, err := exec.LookPath("tmux"); err == nil {
		return "tmux"
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
func runMultiWezTerm(args *MultiArgs, username, proxyHost, keyPath string) error {
	if os.Getenv("TERM_PROGRAM") != "WezTerm" {
		return fmt.Errorf("backend wezterm richiede di essere dentro WezTerm")
	}

	weztermBin, err := exec.LookPath("wezterm")
	if err != nil {
		return fmt.Errorf("wezterm CLI non trovato nel PATH")
	}

	sogarkExe, _ := os.Executable()

	var paneIDs []string

	// Create split panes for each host
	for i, h := range args.Hosts {
		splitDir := "--bottom"
		if i == 0 {
			splitDir = "--right" // first host to the right of broadcaster
		}
		splitArgs := []string{"cli", "split-pane", splitDir, "--",
			sogarkExe, "ssh", h.TargetUser + "@" + h.Address}

		cmd := exec.Command(weztermBin, splitArgs...)
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("errore split-pane per %s: %w", h.Name, err)
		}
		paneID := strings.TrimSpace(string(out))
		paneIDs = append(paneIDs, paneID)
	}

	fmt.Printf("[+] WezTerm: %d pane SSH aperti\n", len(args.Hosts))
	for i, h := range args.Hosts {
		fmt.Printf("    [pane %s] %s (%s@%s)\n", paneIDs[i], h.Name, h.TargetUser, h.Address)
	}

	if !args.Sync {
		fmt.Println("[i] Input non sincronizzato (--no-sync)")
		return nil
	}

	return weztermBroadcastLoop(weztermBin, paneIDs, "")
}

// weztermBroadcastLoop reads lines from stdin and sends them to all panes.
func weztermBroadcastLoop(weztermBin string, paneIDs []string, initialCmd string) error {
	// Send initial command if provided (for exec)
	if initialCmd != "" {
		weztermSendText(weztermBin, paneIDs, initialCmd+"\n")
		fmt.Printf("[+] Comando inviato: %s\n", initialCmd)
	}

	fmt.Println("[+] Broadcast attivo. Digita comandi (Ctrl+D per uscire):")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("[sogark] > ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if line == "" {
			// Send just Enter (useful for confirming prompts)
			weztermSendText(weztermBin, paneIDs, "\n")
			continue
		}
		weztermSendText(weztermBin, paneIDs, line+"\n")
	}

	fmt.Println("\n[+] Broadcast terminato. I pane SSH restano attivi.")
	return nil
}

// weztermSendText sends text to all specified WezTerm panes.
func weztermSendText(weztermBin string, paneIDs []string, text string) {
	for _, pid := range paneIDs {
		cmd := exec.Command(weztermBin, "cli", "send-text", "--pane-id", pid, "--no-paste")
		cmd.Stdin = strings.NewReader(text)
		cmd.Run()
	}
}

// RunExec opens interactive SSH sessions, types the command in all panes,
// and stays attached for follow-up commands.
// CyberArk PSMP requires interactive sessions, so BatchMode exec doesn't work.
func RunExec(hosts []HostTarget, command, username, proxyHost, keyPath string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("nessun host specificato")
	}

	backend := detectMultiBackend()

	switch backend {
	case "wezterm":
		return runExecWezTerm(hosts, command, username, proxyHost, keyPath)
	case "tmux":
		return runExecTmux(hosts, command, username, proxyHost, keyPath)
	default:
		return fmt.Errorf("exec richiede tmux o WezTerm (CyberArk richiede sessioni interattive)")
	}
}

func runExecWezTerm(hosts []HostTarget, command, username, proxyHost, keyPath string) error {
	if os.Getenv("TERM_PROGRAM") != "WezTerm" {
		return fmt.Errorf("exec con WezTerm richiede di essere dentro WezTerm")
	}

	weztermBin, err := exec.LookPath("wezterm")
	if err != nil {
		return fmt.Errorf("wezterm CLI non trovato nel PATH")
	}

	sogarkExe, _ := os.Executable()

	var paneIDs []string

	for i, h := range hosts {
		splitDir := "--bottom"
		if i == 0 {
			splitDir = "--right"
		}
		splitArgs := []string{"cli", "split-pane", splitDir, "--",
			sogarkExe, "ssh", h.TargetUser + "@" + h.Address}

		cmd := exec.Command(weztermBin, splitArgs...)
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("errore split-pane per %s: %w", h.Name, err)
		}
		paneIDs = append(paneIDs, strings.TrimSpace(string(out)))
	}

	fmt.Printf("[+] WezTerm exec: %d pane SSH aperti\n", len(hosts))
	for i, h := range hosts {
		fmt.Printf("    [pane %s] %s (%s@%s)\n", paneIDs[i], h.Name, h.TargetUser, h.Address)
	}

	return weztermBroadcastLoop(weztermBin, paneIDs, command)
}

func runExecTmux(hosts []HostTarget, command, username, proxyHost, keyPath string) error {

	sessionName := "sogark-exec"

	// Kill any pre-existing session with same name
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Create first pane
	first := hosts[0]
	sshCmd := buildSSHCmd(username, first.TargetUser, first.Address, proxyHost, keyPath)
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, sshCmd)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("errore creazione sessione tmux: %w", err)
	}

	// Add remaining hosts as split panes
	for _, h := range hosts[1:] {
		sshCmd = buildSSHCmd(username, h.TargetUser, h.Address, proxyHost, keyPath)
		cmd = exec.Command("tmux", "split-window", "-t", sessionName, sshCmd)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("errore aggiunta pane per %s: %w", h.Name, err)
		}
		exec.Command("tmux", "select-layout", "-t", sessionName, "tiled").Run()
	}

	// Enable synchronize-panes
	exec.Command("tmux", "set-window-option", "-t", sessionName, "synchronize-panes", "on").Run()

	// Send the command to all panes (synchronized)
	exec.Command("tmux", "send-keys", "-t", sessionName, command, "Enter").Run()

	fmt.Printf("[+] Comando inviato a %d host: %s\n", len(hosts), command)
	fmt.Println("    Ctrl+B poi :kill-session per chiudere")

	// Attach to see output
	attachCmd := exec.Command("tmux", "attach-session", "-t", sessionName)
	attachCmd.Stdin = os.Stdin
	attachCmd.Stdout = os.Stdout
	attachCmd.Stderr = os.Stderr
	return attachCmd.Run()
}
