package hosts

import (
	"fmt"
	"os"
	"strings"

	msg "github.com/sogei/cyberark-cli/internal/messages"
)

const (
	sshConfigMarkerStart = "# --- sogark:%s ---"
	sshConfigMarkerEnd   = "# --- /sogark:%s ---"
)

// SSHConfigPath returns the path to ~/.ssh/config.
func SSHConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home + "/.ssh/config", nil
}

// UpdateSSHConfig adds or updates a sogark-managed entry in ~/.ssh/config.
func UpdateSSHConfig(host *Host, username, proxyHost, keyPath string) error {
	configPath, err := SSHConfigPath()
	if err != nil {
		return err
	}

	// Ensure ~/.ssh directory exists
	dir := configPath[:len(configPath)-len("/config")]
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf(msg.SSHConfigMkdir, err)
	}

	targetUser := host.User
	if targetUser == "" {
		targetUser = "root"
	}

	entry := buildSSHConfigEntry(host.Name, host.Address, username, targetUser, proxyHost, keyPath)

	// Read existing config
	existing, _ := os.ReadFile(configPath)
	content := string(existing)

	// Remove existing sogark entry for this host
	content = removeSSHConfigEntry(content, host.Name)

	// Append new entry
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry

	return os.WriteFile(configPath, []byte(content), 0600)
}

// RemoveSSHConfig removes a sogark-managed entry from ~/.ssh/config.
func RemoveSSHConfig(hostName string) error {
	configPath, err := SSHConfigPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	content := removeSSHConfigEntry(string(data), hostName)
	return os.WriteFile(configPath, []byte(content), 0600)
}

func buildSSHConfigEntry(name, address, username, targetUser, proxyHost, keyPath string) string {
	start := fmt.Sprintf(sshConfigMarkerStart, name)
	end := fmt.Sprintf(sshConfigMarkerEnd, name)
	user := fmt.Sprintf("%s@%s@%s", username, targetUser, address)

	return fmt.Sprintf(`%s
Host %s
    HostName %s
    User %s
    IdentityFile %s
%s
`, start, name, proxyHost, user, keyPath, end)
}

func removeSSHConfigEntry(content, hostName string) string {
	start := fmt.Sprintf(sshConfigMarkerStart, hostName)
	end := fmt.Sprintf(sshConfigMarkerEnd, hostName)

	startIdx := strings.Index(content, start)
	if startIdx < 0 {
		return content
	}
	endIdx := strings.Index(content[startIdx:], end)
	if endIdx < 0 {
		return content
	}
	endIdx = startIdx + endIdx + len(end)

	// Also remove trailing newline
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	return content[:startIdx] + content[endIdx:]
}
