//go:build !windows

package terminal

import "os"

func EnableANSI(output *os.File) (func() error, error) {
	return func() error { return nil }, nil
}
