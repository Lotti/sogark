package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Lotti/sogark/internal/config"
	msg "github.com/Lotti/sogark/internal/messages"
)

const (
	versionCacheFile    = ".version_cache"
	versionCheckEvery   = 24 * time.Hour
	versionCheckTimeout = 10 * time.Second
)

type versionCache struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
}

func versionCachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, versionCacheFile), nil
}

func readVersionCache() (*versionCache, error) {
	path, err := versionCachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c versionCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func writeVersionCache(c *versionCache) {
	path, err := versionCachePath()
	if err != nil {
		return
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
}

// notifyIfUpdateAvailable prints a banner to stderr if the cached latest
// version is newer than the running binary. Fully silent on any error.
func notifyIfUpdateAvailable() {
	if version == "dev" {
		return
	}
	c, err := readVersionCache()
	if err != nil || c.LatestVersion == "" {
		return
	}
	if isNewerVersion(c.LatestVersion, version) {
		fmt.Fprintf(os.Stderr, msg.VersionUpdateAvailable, c.LatestVersion, version)
	}
}

// runBackgroundVersionCheck spawns a goroutine that fetches the latest version
// at most once every 24 hours and writes the result to the cache file.
// The notification is shown on the next invocation. All errors are silent.
func runBackgroundVersionCheck() {
	if version == "dev" {
		return
	}

	// Skip if checked recently enough.
	c, _ := readVersionCache()
	if c != nil && time.Since(c.LastCheck) < versionCheckEvery {
		return
	}

	go func() {
		cfg, err := config.LoadOrDefaults()
		if err != nil {
			return
		}
		repo := cfg.ResolvedUpdateRepo()
		if repo == "" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), versionCheckTimeout)
		defer cancel()

		latest, err := fetchLatestVersion(ctx, &http.Client{Timeout: versionCheckTimeout}, repo)
		if err != nil || latest == "" {
			return
		}

		writeVersionCache(&versionCache{
			LastCheck:     time.Now(),
			LatestVersion: latest,
		})
	}()
}

// isNewerVersion reports whether candidate semver is higher than current.
// Both must be in vMAJOR.MINOR.PATCH format; returns false on parse error.
func isNewerVersion(candidate, current string) bool {
	cv := parseSemver(candidate)
	cc := parseSemver(current)
	if cv == nil || cc == nil {
		return false
	}
	for i := range cv {
		if cv[i] > cc[i] {
			return true
		}
		if cv[i] < cc[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release suffix (e.g. "1.2.3-beta")
	v = strings.SplitN(v, "-", 2)[0]
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return nums
}
