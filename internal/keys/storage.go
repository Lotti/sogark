package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	msg "github.com/sogei/cyberark-cli/internal/messages"
)

// FileNames returns the key file names for a given base name.
func FileNames(baseName string) (openssh, pem, ppk string) {
	return baseName, baseName + ".pem", baseName + ".ppk"
}

// SaveResult describes a saved key file.
type SaveResult struct {
	Format string
	Path   string
}

// Save writes the parsed keys to the specified directory.
// Only keys matching the requested formats are saved.
func Save(parsed *Parsed, dir string, baseName string, formats []string) ([]SaveResult, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf(msg.KeysMkdirErr, dir, err)
	}

	opensshName, pemName, ppkName := FileNames(baseName)
	wantFormat := makeFormatSet(formats)

	var results []SaveResult

	if parsed.OpenSSH != "" && wantFormat["openssh"] {
		path := filepath.Join(dir, opensshName)
		if err := writeKeyFile(path, parsed.OpenSSH); err != nil {
			return nil, err
		}
		results = append(results, SaveResult{Format: "OpenSSH", Path: path})
	}

	if parsed.PEM != "" && wantFormat["pem"] {
		path := filepath.Join(dir, pemName)
		if err := writeKeyFile(path, parsed.PEM); err != nil {
			return nil, err
		}
		results = append(results, SaveResult{Format: "PEM", Path: path})
	}

	if parsed.PPK != "" && wantFormat["ppk"] {
		path := filepath.Join(dir, ppkName)
		if err := writeKeyFile(path, parsed.PPK); err != nil {
			return nil, err
		}
		results = append(results, SaveResult{Format: "PPK", Path: path})
	}

	return results, nil
}

// Clean removes sogark-generated key files from the specified directory.
func Clean(dir string, baseName string) ([]string, error) {
	opensshName, pemName, ppkName := FileNames(baseName)
	candidates := []string{opensshName, pemName, ppkName, ".key_timestamp"}

	var removed []string
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err == nil {
			removed = append(removed, name)
		} else if !os.IsNotExist(err) {
			return removed, fmt.Errorf(msg.KeysRemoveErr, path, err)
		}
	}
	return removed, nil
}

func writeKeyFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf(msg.KeysWriteErr, path, err)
	}
	return nil
}

func makeFormatSet(formats []string) map[string]bool {
	set := make(map[string]bool)
	for _, f := range formats {
		set[strings.ToLower(strings.TrimSpace(f))] = true
	}
	return set
}
