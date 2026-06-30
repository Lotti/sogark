package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	msg "github.com/Lotti/sogark/internal/messages"
)

func fetchChecksums(ctx context.Context, httpClient *http.Client, repo, version string) (map[string]string, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/checksums.txt", repo, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sogark-updater")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(msg.UpdateHTTPErr, resp.StatusCode, url)
	}

	return parseChecksums(resp.Body)
}

func parseChecksums(r io.Reader) (map[string]string, error) {
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid checksum line: %q", line)
		}

		name := strings.TrimPrefix(parts[len(parts)-1], "*")
		checksums[name] = strings.ToLower(parts[0])
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return checksums, nil
}

func verifyFileChecksum(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("expected %s, got %s", expected, actual)
	}

	return nil
}
