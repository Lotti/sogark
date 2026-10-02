package auth

import (
	"errors"
	"io"
	"os"

	"github.com/Lotti/sogark/internal/terminal"
)

func writeTerminalQR(output io.Writer, rendered string) (err error) {
	if file, ok := output.(*os.File); ok {
		restore, setupErr := terminal.EnableANSI(file)
		if setupErr != nil {
			return setupErr
		}
		defer func() { err = errors.Join(err, restore()) }()
	}
	_, err = io.WriteString(output, rendered)
	return err
}
