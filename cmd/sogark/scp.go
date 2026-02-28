package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/hosts"
	"github.com/sogei/cyberark-cli/internal/keys"
	sshpkg "github.com/sogei/cyberark-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func newScpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scp [sogark-flags] source... target",
		Short: "Trasferimento file via SCP attraverso PSMP",
		Long: `Wrapper trasparente per scp: sogark inietta la chiave SSH (-i) e traduce i path remoti nel formato PSMP.

I path remoti (host:path o user@host:path) vengono riscritti automaticamente:
  host:/path  →  corp@target@host@psmp:/path

Flag sogark (--dry-run, --force-login, -u, --key-format, --tag, --any-tag) devono precedere i flag scp.
Tutti gli altri flag vengono passati direttamente a scp.

Se la chiave SSH è scaduta, viene eseguita l'autenticazione automatica.

Modalità batch con --tag/--any-tag: invia file a tutti gli host del tag.
Usare ":/path" per indicare il percorso remoto su ogni host.`,
		Example: `  # Upload file
  sogark scp file.txt 10.1.2.3:/tmp/

  # Upload con tag inline
  sogark scp file.txt #webservers:/tmp/
  sogark scp file.txt oper1@#web#prod:/tmp/

  # Download con tag inline (crea sottocartelle per host)
  sogark scp #webservers:/etc/hosts ./configs/

  # Upload a tutti gli host con flag --tag
  sogark scp --tag webservers file.txt :/tmp/

  # Upload directory a tag multipli (OR)
  sogark scp --any-tag web,app -r ./deploy :/opt/app/

  # Upload directory
  sogark scp -r ./mydir 10.1.2.3:/opt/

  # Download file
  sogark scp 10.1.2.3:/etc/hosts ./

  # Con utente target specifico
  sogark scp file.txt admin@10.1.2.3:/tmp/

  # Con flag scp nativi (compressione, verbose, porta)
  sogark scp -C -v -P 2222 file.txt 10.1.2.3:/tmp/

  # Dry run (mostra comando senza eseguirlo)
  sogark scp --dry-run file.txt 10.1.2.3:/tmp/`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Manually parse sogark-specific flags; everything else goes to scp.
			sf, err := parseScpFlags(args)
			if err != nil {
				return err
			}
			if len(sf.passArgs) == 0 {
				return fmt.Errorf("specificare source e target\nEsempio: sogark scp file.txt host:/tmp/")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			targetUser := cfg.DefaultTargetUser
			if sf.user != "" {
				targetUser = sf.user
			}

			keyDir, err := cfg.ResolveKeyDir()
			if err != nil {
				return err
			}

			// Check key validity
			valid, remaining, _ := keys.IsValid(keyDir, cfg.SSHKeyName, cfg.KeyTTLHours)
			if valid && !sf.forceLogin {
				fmt.Printf("[+] Chiave valida (scade tra %s)\n", formatDuration(remaining))
			} else {
				if !valid {
					fmt.Println("[!] Chiave scaduta o assente, avvio autenticazione...")
				}
				if err := doLogin(cfg); err != nil {
					return err
				}
			}

			keyName := cfg.SSHKeyName
			if sf.keyFormat == "pem" {
				keyName += ".pem"
			}
			keyPath := filepath.Join(keyDir, keyName)

			// Detect #tag syntax in remote path args
			if sf.tag == "" && sf.anyTag == "" {
				sf.passArgs, sf.tag, sf.user, sf.downloadDir = extractScpTagArgs(sf.passArgs, sf.user)
				if sf.user != "" {
					targetUser = sf.user
				}
			}

			// Batch mode: --tag, --any-tag, or #tag syntax
			if sf.tag != "" || sf.anyTag != "" {
				targets, err := resolveTargets(cfg, nil, sf.tag, sf.anyTag)
				if err != nil {
					return err
				}
				if sf.user != "" {
					for i := range targets {
						targets[i].TargetUser = sf.user
					}
				}

				batchArgs := &sshpkg.BatchScpArgs{
					Username:    cfg.Username,
					ProxyHost:   cfg.ProxyHost,
					KeyPath:     keyPath,
					ScpArgs:     sf.passArgs,
					Hosts:       targets,
					Parallel:    3,
					DownloadDir: sf.downloadDir,
				}

				if sf.dryRun {
					for _, h := range targets {
						expanded := sshpkg.ExpandBatchRemote(sf.passArgs, h)
						localDir := sf.downloadDir
						if localDir != "" {
							localDir = filepath.Join(localDir, h.Name)
						}
						scpA := &sshpkg.ScpArgs{
							Username: cfg.Username, TargetUser: h.TargetUser,
							ProxyHost: cfg.ProxyHost, KeyPath: keyPath, ScpArgs: expanded,
						}
						cmdStr := scpA.CommandString()
						if localDir != "" {
							cmdStr += " → " + localDir
						}
						fmt.Printf("[%s] > %s\n", h.Name, cmdStr)
					}
					return nil
				}

				return sshpkg.RunBatchScp(batchArgs)
			}

			// Single-host mode
			sogarkDir, _ := config.Dir()
			reg, _ := hosts.NewRegistry(sogarkDir)

			resolvedArgs := resolveScpArgs(sf.passArgs, reg, targetUser)

			scpArgs := &sshpkg.ScpArgs{
				Username:   cfg.Username,
				TargetUser: targetUser,
				ProxyHost:  cfg.ProxyHost,
				KeyPath:    keyPath,
				ScpArgs:    resolvedArgs,
			}

			fmt.Printf("> %s\n", scpArgs.CommandString())

			if sf.dryRun {
				return nil
			}

			return scpArgs.Run()
		},
	}

	return cmd
}

// scpFlags holds parsed sogark-specific flags for the scp command.
type scpFlags struct {
	user        string
	keyFormat   string
	tag         string
	anyTag      string
	forceLogin  bool
	dryRun      bool
	downloadDir string   // set when #tag download detected
	passArgs    []string
}

// parseScpFlags separates sogark-specific flags from scp passthrough args.
// Sogark flags: --dry-run, --force-login, -u/--user, --key-format, --tag, --any-tag, --verbose, -h/--help.
func parseScpFlags(args []string) (sf scpFlags, err error) {
	sf.keyFormat = "openssh"
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--verbose":
			os.Setenv("SOGARK_DEBUG", "1")
		case a == "--dry-run":
			sf.dryRun = true
		case a == "--force-login":
			sf.forceLogin = true
		case a == "-u" || a == "--user":
			i++
			if i >= len(args) {
				err = fmt.Errorf("flag %s richiede un valore", a)
				return
			}
			sf.user = args[i]
		case strings.HasPrefix(a, "--user="):
			sf.user = strings.TrimPrefix(a, "--user=")
		case a == "--key-format":
			i++
			if i >= len(args) {
				err = fmt.Errorf("flag %s richiede un valore", a)
				return
			}
			sf.keyFormat = args[i]
		case strings.HasPrefix(a, "--key-format="):
			sf.keyFormat = strings.TrimPrefix(a, "--key-format=")
		case a == "--tag":
			i++
			if i >= len(args) {
				err = fmt.Errorf("flag --tag richiede un valore")
				return
			}
			sf.tag = args[i]
		case strings.HasPrefix(a, "--tag="):
			sf.tag = strings.TrimPrefix(a, "--tag=")
		case a == "--any-tag":
			i++
			if i >= len(args) {
				err = fmt.Errorf("flag --any-tag richiede un valore")
				return
			}
			sf.anyTag = args[i]
		case strings.HasPrefix(a, "--any-tag="):
			sf.anyTag = strings.TrimPrefix(a, "--any-tag=")
		case a == "-h" || a == "--help":
			err = fmt.Errorf("help")
			return
		case a == "--":
			// explicit separator: everything after goes to scp
			sf.passArgs = append(sf.passArgs, args[i+1:]...)
			i = len(args)
		default:
			sf.passArgs = append(sf.passArgs, a)
		}
		i++
	}
	return
}

// resolveScpArgs resolves host names from the registry in remote path arguments.
// For example, "myserver:/path" becomes "10.1.2.3:/path" if myserver is registered.
func resolveScpArgs(args []string, reg *hosts.Registry, defaultTargetUser string) []string {
	if reg == nil {
		return args
	}
	resolved := make([]string, len(args))
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") {
			resolved[i] = arg
			continue
		}
		host, path, ok := sshpkg.ParseRemotePath(arg)
		if !ok {
			resolved[i] = arg
			continue
		}
		// Strip user@ prefix for registry lookup
		lookupHost := host
		userPrefix := ""
		if idx := strings.Index(host, "@"); idx >= 0 {
			userPrefix = host[:idx+1]
			lookupHost = host[idx+1:]
		}
		if h, found := reg.Get(lookupHost); found {
			if userPrefix == "" && h.User != "" {
				userPrefix = h.User + "@"
			}
			resolved[i] = userPrefix + h.Address + ":" + path
		} else {
			resolved[i] = arg
		}
	}
	return resolved
}

// extractScpTagArgs scans scp args for #tag patterns in remote paths.
// If found, extracts tags → sets tag string (comma-separated), rewrites the arg to ":/path",
// and detects upload vs download for batch mode.
// Returns modified args, tag string, user override, and downloadDir (non-empty for downloads).
func extractScpTagArgs(args []string, existingUser string) (newArgs []string, tag, user, downloadDir string) {
	newArgs = make([]string, len(args))
	copy(newArgs, args)

	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		host, path, isRemote := sshpkg.ParseRemotePath(a)
		if !isRemote {
			continue
		}

		// Extract user@ prefix if present
		hostPart := host
		userPart := ""
		if idx := strings.Index(host, "@"); idx >= 0 {
			userPart = host[:idx]
			hostPart = host[idx+1:]
		}

		if !strings.HasPrefix(hostPart, "#") {
			continue
		}

		// Parse tags from #tag1#tag2#tag3
		tagParts := strings.Split(hostPart, "#")
		var tags []string
		for _, p := range tagParts {
			if p != "" {
				tags = append(tags, p)
			}
		}
		if len(tags) == 0 {
			continue
		}

		tag = strings.Join(tags, ",")
		if userPart != "" && existingUser == "" {
			user = userPart
		}

		// Rewrite arg to ":/path" for batch expansion
		newArgs[i] = ":" + path

		// Detect download: #tag arg is NOT the last non-flag arg
		isLast := true
		for j := i + 1; j < len(args); j++ {
			if !strings.HasPrefix(args[j], "-") {
				isLast = false
				break
			}
		}
		if !isLast {
			// Upload: #tag is source, local is target — wait, this is wrong.
			// In SCP, the LAST arg is always the target.
			// If #tag is NOT the last non-flag arg, #tag is a SOURCE → download.
			// Find the last non-flag arg as the local destination.
			for j := len(args) - 1; j > i; j-- {
				if !strings.HasPrefix(args[j], "-") {
					downloadDir = args[j]
					break
				}
			}
		}

		break // only process first #tag match
	}
	return
}
