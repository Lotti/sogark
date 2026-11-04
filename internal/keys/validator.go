package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	msg "github.com/Lotti/sogark/internal/messages"
)

const timestampFile = ".key_timestamp"

// SaveTimestamp records the current time in the key directory.
func SaveTimestamp(dir string) error {
	path := filepath.Join(dir, timestampFile)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	return os.WriteFile(path, []byte(ts), 0600)
}

// IsValid checks the requested key files and their TTL; the default is OpenSSH.
func IsValid(dir, baseName string, ttlHours int, formats ...string) (bool, time.Duration, error) {
	if len(formats) == 0 {
		formats = []string{"OpenSSH"}
	}
	openssh, pem, ppk := FileNames(baseName)
	names := map[string]string{"openssh": openssh, "pem": pem, "ppk": ppk}
	for _, format := range formats {
		name, ok := names[strings.ToLower(strings.TrimSpace(format))]
		if !ok {
			return false, 0, fmt.Errorf("unsupported key format %q", format)
		}
		info, err := os.Stat(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		if err != nil {
			return false, 0, fmt.Errorf("inspect cached %s key: %w", format, err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return false, 0, nil
		}
	}

	// Check timestamp
	tsPath := filepath.Join(dir, timestampFile)
	data, err := os.ReadFile(tsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf(msg.KeysReadTSErr, err)
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
