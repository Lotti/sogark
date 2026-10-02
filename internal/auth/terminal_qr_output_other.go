//go:build !windows

package auth

import "io"

func writeTerminalQR(output io.Writer, rendered string) error {
	_, err := io.WriteString(output, rendered)
	return err
}
