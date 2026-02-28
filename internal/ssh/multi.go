package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// MultiArgs holds parameters for a multi-pane session.
type MultiArgs struct {
	SessionName string
	Hosts       []HostTarget
	Sync        bool
	Backend     string // "auto", "tmux", "wt" (Windows Terminal)
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
	case "wt":
		return runMultiWT(args, username, proxyHost, keyPath)
	case "tmux":
		return runMultiTmux(args, username, proxyHost, keyPath)
	default:
		return fmt.Errorf("backend %q non supportato (usa 'wt' o 'tmux')", backend)
	}
}

// detectMultiBackend selects the best available multi-pane backend.
func detectMultiBackend() string {
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

// RunExec opens interactive SSH sessions via tmux, types the command in all
// panes (synchronized), and attaches so the user can see output.
// CyberArk PSMP requires interactive sessions, so BatchMode exec doesn't work.
func RunExec(hosts []HostTarget, command, username, proxyHost, keyPath string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("nessun host specificato")
	}

	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("exec richiede tmux (CyberArk richiede sessioni interattive).\n" +
			"  macOS:  brew install tmux\n" +
			"  Linux:  sudo apt install tmux")
	}

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
