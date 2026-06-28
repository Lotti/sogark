package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

// ConnectArgs holds the parameters for an SSH connection.
type ConnectArgs struct {
	Username   string   // corporate user (e.g. mario.rossi)
	TargetUser string   // user on remote machine (e.g. root)
	Host       string   // target host IP or hostname
	ProxyHost  string   // PSMP proxy hostname
	KeyPath    string   // path to private key
	ExtraArgs  []string // additional SSH arguments
}

// CommandLine returns the full SSH command as a string slice.
func (a *ConnectArgs) CommandLine() []string {
	user := fmt.Sprintf("%s@%s@%s@%s", a.Username, a.TargetUser, a.Host, a.ProxyHost)
	args := []string{"ssh", user, "-i", a.KeyPath, "-o", "IdentitiesOnly=yes"}
	args = append(args, a.ExtraArgs...)
	return args
}

// CommandString returns the SSH command as a printable string.
func (a *ConnectArgs) CommandString() string {
	return strings.Join(a.CommandLine(), " ")
}

// Exec connects to the remote host via SSH.
// On Unix it replaces the current process; on Windows it runs as a subprocess.
func (a *ConnectArgs) Exec() error {
	if runtime.GOOS == "windows" {
		return a.Run()
	}
	args := a.CommandLine()
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("client ssh non trovato: %w", err)
	}
	return syscall.Exec(sshPath, args, os.Environ())
}

// Run starts the SSH command as a subprocess and waits for completion.
func (a *ConnectArgs) Run() error {
	args := a.CommandLine()

	// On Windows, prefer ssh.exe to avoid MinGW64/MSYS2 resolving to
	// a Cygwin-style ssh that may behave differently.
	cmdName := args[0]
	if runtime.GOOS == "windows" {
		if sshExe, err := exec.LookPath("ssh.exe"); err == nil {
			cmdName = sshExe
		}
	}

	cmd := exec.Command(cmdName, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ParseTarget parses a "[user@]host" string into target user and host.
func ParseTarget(target, defaultUser string) (user, host string) {
	if idx := strings.Index(target, "@"); idx >= 0 {
		return target[:idx], target[idx+1:]
	}
	return defaultUser, target
}
