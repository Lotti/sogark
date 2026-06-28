package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Lotti/sogark/internal/config"
	msg "github.com/Lotti/sogark/internal/messages"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var (
		targetVersion string
		force         bool
		checkOnly     bool
	)

	cmd := &cobra.Command{
		Use:     "update",
		Short:   msg.UpdateShort,
		Long:    msg.UpdateLong,
		Example: msg.UpdateExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadOrDefaults()
			if err != nil {
				return err
			}

			repo := cfg.ResolvedUpdateRepo()
			httpClient := &http.Client{Timeout: 30 * time.Second}

			if targetVersion == "" {
				fmt.Println(msg.UpdateCheckingVersion)
				latest, err := fetchLatestVersion(signalCtx, httpClient, repo)
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

			downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, targetVersion, sogarkBinaryName())
			fmt.Printf("[*] Download: %s\n", downloadURL)

			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf(msg.UpdateErrExecPath, err)
			}
			execPath, err = filepath.EvalSymlinks(execPath)
			if err != nil {
				return fmt.Errorf(msg.UpdateErrSymlink, err)
			}

			tmpPath := execPath + ".update"
			if err := downloadFile(signalCtx, httpClient, downloadURL, tmpPath); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf(msg.UpdateErrDownload, err)
			}

			replaceResult, err := replaceCurrentBinary(execPath, tmpPath, targetVersion)
			if err != nil {
				os.Remove(tmpPath)
				return err
			}

			if replaceResult.Deferred {
				fmt.Printf(msg.UpdateDeferredSuccess, targetVersion)
				return nil
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

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func fetchLatestVersion(ctx context.Context, httpClient *http.Client, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sogark-updater")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(msg.UpdateHTTPErrVersion, resp.StatusCode, url)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

func downloadFile(ctx context.Context, httpClient *http.Client, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "sogark-updater")

	resp, err := httpClient.Do(req)
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
