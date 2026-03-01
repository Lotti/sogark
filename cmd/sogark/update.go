package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var (
		targetVersion string
		force         bool
		checkOnly     bool
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Aggiorna sogark all'ultima versione da Nexus",
		Long: `Controlla la versione più recente disponibile sul repository Nexus
e aggiorna il binario corrente se necessario.

Richiede che nexus_url e nexus_repo siano configurati:
  sogark config set nexus_url https://nexus.example.com
  sogark config set nexus_repo sogark-releases`,
		Example: `  sogark update              # aggiorna all'ultima versione
  sogark update --check      # controlla senza aggiornare
  sogark update --version v1.2.0  # installa versione specifica
  sogark update --force      # forza re-download anche se aggiornato`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if cfg.NexusURL == "" || cfg.NexusRepo == "" {
				return fmt.Errorf("nexus_url e nexus_repo non configurati.\nEsegui:\n  sogark config set nexus_url https://nexus.example.com\n  sogark config set nexus_repo sogark-releases")
			}

			baseURL := strings.TrimRight(cfg.NexusURL, "/") + "/repository/" + cfg.NexusRepo

			// Determine target version
			if targetVersion == "" {
				fmt.Println("[*] Controllo ultima versione disponibile...")
				latest, err := fetchLatestVersion(baseURL)
				if err != nil {
					return fmt.Errorf("errore recupero versione: %w", err)
				}
				targetVersion = latest
			}

			fmt.Printf("[*] Versione corrente: %s\n", version)
			fmt.Printf("[*] Versione disponibile: %s\n", targetVersion)

			if !force && targetVersion == version {
				fmt.Println("[✓] Già aggiornato.")
				return nil
			}

			if checkOnly {
				if targetVersion != version {
					fmt.Println("[!] Aggiornamento disponibile. Esegui 'sogark update' per aggiornare.")
				}
				return nil
			}

			// Determine binary name for this platform
			binaryName := sogarkBinaryName()
			versionPath := "latest"
			if targetVersion != "" {
				versionPath = targetVersion
			}
			downloadURL := baseURL + "/" + versionPath + "/" + binaryName

			fmt.Printf("[*] Download: %s\n", downloadURL)

			// Download to temp file
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("impossibile determinare il percorso dell'eseguibile: %w", err)
			}
			execPath, err = filepath.EvalSymlinks(execPath)
			if err != nil {
				return fmt.Errorf("impossibile risolvere symlink: %w", err)
			}

			tmpPath := execPath + ".update"
			if err := downloadFile(downloadURL, tmpPath); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("errore download: %w", err)
			}

			// Make executable (no-op on Windows)
			if runtime.GOOS != "windows" {
				if err := os.Chmod(tmpPath, 0755); err != nil {
					os.Remove(tmpPath)
					return fmt.Errorf("errore chmod: %w", err)
				}
			}

			// Replace current binary
			if err := os.Rename(tmpPath, execPath); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("errore sostituzione binario: %w", err)
			}

			fmt.Printf("[✓] Aggiornato a %s\n", targetVersion)
			return nil
		},
	}

	cmd.Flags().StringVar(&targetVersion, "version", "", "versione specifica da installare (es. v1.2.0)")
	cmd.Flags().BoolVar(&force, "force", false, "forza il download anche se la versione è uguale")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "controlla senza aggiornare")

	return cmd
}

// fetchLatestVersion reads version.txt from the Nexus latest/ folder.
func fetchLatestVersion(baseURL string) (string, error) {
	resp, err := http.Get(baseURL + "/latest/version.txt")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d da %s/latest/version.txt", resp.StatusCode, baseURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}

// sogarkBinaryName returns the expected binary filename for this platform.
func sogarkBinaryName() string {
	name := fmt.Sprintf("sogark-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// downloadFile downloads a URL to a local file path.
func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d da %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
