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
	msg "github.com/sogei/cyberark-cli/internal/messages"
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
		Short: msg.UpdateShort,
		Long:  msg.UpdateLong,
		Example: msg.UpdateExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if cfg.NexusURL == "" || cfg.NexusRepo == "" {
				return fmt.Errorf(msg.UpdateErrNotConfigured)
			}

			baseURL := strings.TrimRight(cfg.NexusURL, "/") + "/repository/" + cfg.NexusRepo

			// Determine target version
			if targetVersion == "" {
				fmt.Println(msg.UpdateCheckingVersion)
				latest, err := fetchLatestVersion(baseURL)
				if err != nil {
					return fmt.Errorf(msg.UpdateErrFetchVersion, err)
				}
				targetVersion = latest
			}

			fmt.Printf(msg.UpdateCurrentVersion, version)
			fmt.Printf(msg.UpdateAvailableVersion, targetVersion)

			if !force && targetVersion == version {
				fmt.Println(msg.UpdateAlreadyUpToDate)
				return nil
			}

			if checkOnly {
				if targetVersion != version {
					fmt.Println(msg.UpdateAvailable)
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
				return fmt.Errorf(msg.UpdateErrExecPath, err)
			}
			execPath, err = filepath.EvalSymlinks(execPath)
			if err != nil {
				return fmt.Errorf(msg.UpdateErrSymlink, err)
			}

			tmpPath := execPath + ".update"
			if err := downloadFile(downloadURL, tmpPath); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf(msg.UpdateErrDownload, err)
			}

			// Make executable (no-op on Windows)
			if runtime.GOOS != "windows" {
				if err := os.Chmod(tmpPath, 0755); err != nil {
					os.Remove(tmpPath)
					return fmt.Errorf(msg.UpdateErrChmod, err)
				}
			}

			// Replace current binary
			if err := os.Rename(tmpPath, execPath); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf(msg.UpdateErrReplace, err)
			}

			fmt.Printf(msg.UpdateSuccess, targetVersion)
			return nil
		},
	}

	cmd.Flags().StringVar(&targetVersion, "version", "", msg.UpdateFlagVersion)
	cmd.Flags().BoolVar(&force, "force", false, msg.UpdateFlagForce)
	cmd.Flags().BoolVar(&checkOnly, "check", false, msg.UpdateFlagCheck)

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
		return "", fmt.Errorf(msg.UpdateHTTPErrVersion, resp.StatusCode, baseURL)
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
		return fmt.Errorf(msg.UpdateHTTPErr, resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
