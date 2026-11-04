//go:build !windows

package keys

import "os"

func protectKeyFile(path string) error {
	return os.Chmod(path, 0600)
}
