package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ScpArgs holds the parameters for an SCP file transfer via PSMP.
type ScpArgs struct {
	Username   string   // corporate user
	TargetUser string   // user on remote machine
	ProxyHost  string   // PSMP proxy hostname
	KeyPath    string   // path to private key
	ScpArgs    []string // native scp arguments + source/target (everything after --)
}

// CommandLine returns the full scp command as a string slice.
// Remote paths matching [user@]host:path are rewritten to PSMP format.
func (a *ScpArgs) CommandLine() []string {
	args := []string{"scp", "-i", a.KeyPath, "-o", "IdentitiesOnly=yes"}

	// OpenSSH >= 9.0 defaults to SFTP protocol; CyberArk PSMP doesn't
	// support it. Use -O to force legacy SCP protocol.
	if scpNeedsLegacyFlag() {
		args = append(args, "-O")
	}

	for _, arg := range a.ScpArgs {
		args = append(args, a.rewriteRemote(arg))
	}
	return args
}

// CommandString returns the scp command as a printable string.
func (a *ScpArgs) CommandString() string {
	return strings.Join(a.CommandLine(), " ")
}

// Run starts the scp command as a subprocess and waits for completion.
func (a *ScpArgs) Run() error {
	args := a.CommandLine()

	cmdName := args[0]
	if runtime.GOOS == "windows" {
		if scpExe, err := exec.LookPath("scp.exe"); err == nil {
			cmdName = scpExe
		}
	}

	cmd := exec.Command(cmdName, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// rewriteRemote checks if s is a remote path (contains host:) and rewrites
// it to PSMP format: corp@target@host@proxy:path.
// It respects user@host:path syntax for target user override.
// Local paths and scp flags (starting with -) are returned unchanged.
func (a *ScpArgs) rewriteRemote(s string) string {
	if strings.HasPrefix(s, "-") {
		return s
	}

	host, path, ok := ParseRemotePath(s)
	if !ok {
		return s
	}

	targetUser := a.TargetUser
	if idx := strings.Index(host, "@"); idx >= 0 {
		targetUser = host[:idx]
		host = host[idx+1:]
	}

	psmpUser := fmt.Sprintf("%s@%s@%s@%s", a.Username, targetUser, host, a.ProxyHost)
	return psmpUser + ":" + path
}

// ParseRemotePath splits a remote path string "host:path" into host and path.
// Returns ("", "", false) for local paths or paths without ":".
// Handles Windows drive letters (C:\...) by checking if host is a single letter.
func ParseRemotePath(s string) (host, path string, ok bool) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return "", "", false
	}
	h := s[:idx]
	if len(h) == 1 && runtime.GOOS == "windows" {
		// Windows drive letter like C:\path
		return "", "", false
	}
	if h == "" {
		return "", "", false
	}
	return h, s[idx+1:], true
}

// HasRemoteArg returns true if any argument looks like a remote path (host:path).
func HasRemoteArg(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if _, _, ok := ParseRemotePath(arg); ok {
			return true
		}
	}
	return false
}

// BatchScpArgs holds parameters for batch SCP to multiple hosts.
type BatchScpArgs struct {
	Username   string
	ProxyHost  string
	KeyPath    string
	ScpArgs    []string // scp flags + local files + ":remotepath"
	Hosts      []HostTarget
	Parallel   int // max concurrent transfers (0 = sequential)
}

// RunBatchScp runs SCP to each host, reporting results per host.
// Remote args using ":/path" (no host) are expanded for each target.
func RunBatchScp(args *BatchScpArgs) error {
	if len(args.Hosts) == 0 {
		return fmt.Errorf("nessun host specificato")
	}

	parallel := args.Parallel
	if parallel <= 0 {
		parallel = 1
	}

	type result struct {
		name   string
		output string
		err    error
	}

	sem := make(chan struct{}, parallel)
	results := make(chan result, len(args.Hosts))

	for _, h := range args.Hosts {
		sem <- struct{}{}
		go func(h HostTarget) {
			defer func() { <-sem }()

			// Expand ":/path" args with this host's address
			expanded := ExpandBatchRemote(args.ScpArgs, h)

			scpArgs := &ScpArgs{
				Username:   args.Username,
				TargetUser: h.TargetUser,
				ProxyHost:  args.ProxyHost,
				KeyPath:    args.KeyPath,
				ScpArgs:    expanded,
			}

			cmdLine := scpArgs.CommandLine()
			cmdName := cmdLine[0]
			if runtime.GOOS == "windows" {
				if scpExe, err := exec.LookPath("scp.exe"); err == nil {
					cmdName = scpExe
				}
			}

			cmd := exec.Command(cmdName, cmdLine[1:]...)
			out, err := cmd.CombinedOutput()
			results <- result{name: h.Name, output: string(out), err: err}
		}(h)
	}

	succeeded := 0
	failed := 0
	for range args.Hosts {
		r := <-results
		if r.err != nil {
			fmt.Printf("[%s] ERRORE: %s", r.name, r.output)
			fmt.Fprintf(os.Stderr, "[%s] %v\n", r.name, r.err)
			failed++
		} else {
			fmt.Printf("[%s] OK\n", r.name)
			succeeded++
		}
	}

	total := len(args.Hosts)
	if failed > 0 {
		fmt.Printf("[!] SCP: %d/%d completati, %d falliti\n", succeeded, total, failed)
	} else {
		fmt.Printf("[+] SCP: %d/%d completati\n", succeeded, total)
	}
	return nil
}

// ExpandBatchRemote replaces ":/path" (bare colon prefix, no host) with
// "[user@]host:/path" using the given HostTarget.
func ExpandBatchRemote(args []string, h HostTarget) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.HasPrefix(a, ":") {
			// Bare remote path → attach host
			out[i] = h.Address + a
		} else {
			out[i] = a
		}
	}
	return out
}

// scpNeedsLegacyFlag detects if the local scp uses SFTP by default (OpenSSH >= 9.0)
// and returns true if -O is needed to force legacy SCP protocol.
func scpNeedsLegacyFlag() bool {
	out, err := exec.Command("ssh", "-V").CombinedOutput()
	if err != nil {
		return false
	}
	s := string(out)
	idx := strings.Index(s, "OpenSSH_")
	if idx < 0 {
		return false
	}
	var major int
	fmt.Sscanf(s[idx+8:], "%d", &major)
	return major >= 9
}
