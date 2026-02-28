package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// MultiArgs holds parameters for a tmux multi-session.
type MultiArgs struct {
	SessionName string
	Hosts       []HostTarget
	Sync        bool
}

// HostTarget represents a single host for multi/exec commands.
type HostTarget struct {
	Name       string
	Address    string
	TargetUser string
}

// RunMulti opens a tmux session with synchronized panes for each host.
func RunMulti(args *MultiArgs, username, proxyHost, keyPath string) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux non trovato. Installalo con:\n" +
			"  macOS:  brew install tmux\n" +
			"  Linux:  sudo apt install tmux")
	}

	if len(args.Hosts) == 0 {
		return fmt.Errorf("nessun host specificato")
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
	return fmt.Sprintf("ssh %s -i %s", user, keyPath)
}

// RunExec executes a command on multiple hosts in parallel and collects output.
func RunExec(hosts []HostTarget, command, username, proxyHost, keyPath string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("nessun host specificato")
	}

	type result struct {
		name   string
		output string
		err    error
	}

	results := make(chan result, len(hosts))

	for _, h := range hosts {
		go func(h HostTarget) {
			user := fmt.Sprintf("%s@%s@%s@%s", username, h.TargetUser, h.Address, proxyHost)
			cmd := exec.Command("ssh", user, "-i", keyPath, "-o", "StrictHostKeyChecking=no", "-o", "BatchMode=yes", command)
			out, err := cmd.CombinedOutput()
			results <- result{name: h.Name, output: string(out), err: err}
		}(h)
	}

	succeeded := 0
	failed := 0
	for range hosts {
		r := <-results
		lines := strings.Split(strings.TrimSpace(r.output), "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Printf("[%s] %s\n", r.name, line)
			}
		}
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "[%s] errore: %v\n", r.name, r.err)
			failed++
		} else {
			succeeded++
		}
	}

	total := len(hosts)
	if failed > 0 {
		fmt.Printf("[!] %d/%d host completati, %d falliti\n", succeeded, total, failed)
	} else {
		fmt.Printf("[+] %d/%d host completati\n", succeeded, total)
	}

	return nil
}
