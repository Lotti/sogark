//go:build !windows

package keys

import "os"

func protectKeyFile(path string) error {
	return os.Chmod(path, 0600)
}

// PrepareForOpenSSH leaves Unix key permissions to OpenSSH.
func PrepareForOpenSSH(string) error {
	return nil
}
