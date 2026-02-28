package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const timestampFile = ".key_timestamp"

// SaveTimestamp records the current time in the key directory.
func SaveTimestamp(dir string) error {
	path := filepath.Join(dir, timestampFile)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	return os.WriteFile(path, []byte(ts), 0600)
}

// IsValid checks if a key exists and is still within the TTL.
func IsValid(dir, baseName string, ttlHours int) (bool, time.Duration, error) {
	// Check that key file exists
	keyPath := filepath.Join(dir, baseName)
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return false, 0, nil
	}

	// Check timestamp
	tsPath := filepath.Join(dir, timestampFile)
	data, err := os.ReadFile(tsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("errore lettura timestamp: %w", err)
	}

	ts, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return false, 0, nil
	}

	created := time.Unix(ts, 0)
	ttl := time.Duration(ttlHours) * time.Hour
	remaining := ttl - time.Since(created)

	if remaining <= 0 {
		return false, 0, nil
	}

	return true, remaining, nil
}
