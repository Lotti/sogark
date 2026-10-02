package terminal

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

func EnableANSI(output *os.File) (func() error, error) {
	if output == nil || !term.IsTerminal(int(output.Fd())) {
		return func() error { return nil }, nil
	}
	handle := windows.Handle(output.Fd())
	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		return nil, fmt.Errorf("read terminal output mode: %w", err)
	}
	mode := original | windows.ENABLE_PROCESSED_OUTPUT | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if mode == original {
		return func() error { return nil }, nil
	}
	if err := windows.SetConsoleMode(handle, mode); err != nil {
		return nil, fmt.Errorf("enable terminal colors and cursor editing: %w", err)
	}
	return func() error {
		if err := windows.SetConsoleMode(handle, original); err != nil {
			return fmt.Errorf("restore terminal output mode: %w", err)
		}
		return nil
	}, nil
}
