//go:build windows

package hosts

import (
	"fmt"

	msg "github.com/sogei/cyberark-cli/internal/messages"
	"golang.org/x/sys/windows/registry"
)

const puttySessionsKey = `Software\SimonTatham\PuTTY\Sessions`

// UpdatePuTTYSession creates or updates a PuTTY session in the Windows registry.
func UpdatePuTTYSession(host *Host, username, proxyHost, keyPath string) error {
	targetUser := host.User
	if targetUser == "" {
		targetUser = "root"
	}

	sessionKey := puttySessionsKey + `\` + host.Name
	key, _, err := registry.CreateKey(registry.CURRENT_USER, sessionKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf(msg.PuTTYCreateSessionErr, err)
	}
	defer key.Close()

	user := fmt.Sprintf("%s@%s@%s", username, targetUser, host.Address)

	values := map[string]string{
		"HostName":      proxyHost,
		"UserName":      user,
		"Protocol":      "ssh",
		"PublicKeyFile": keyPath,
	}

	for name, val := range values {
		if err := key.SetStringValue(name, val); err != nil {
			return fmt.Errorf(msg.PuTTYSetValueErr, name, err)
		}
	}

	return nil
}

// RemovePuTTYSession removes a PuTTY session from the Windows registry.
func RemovePuTTYSession(hostName string) error {
	sessionKey := puttySessionsKey + `\` + hostName
	if err := registry.DeleteKey(registry.CURRENT_USER, sessionKey); err != nil {
		return fmt.Errorf(msg.PuTTYDeleteSessionErr, err)
	}
	return nil
}
