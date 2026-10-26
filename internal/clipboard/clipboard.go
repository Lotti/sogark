package clipboard

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// Write sends text to the user's OS clipboard without placing it in argv.
func Write(ctx context.Context, value string) error {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	case "windows":
		candidates = [][]string{{"clip.exe"}, {"clip"}}
	default:
		candidates = [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard", "-in"}}
	}
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate[0])
		if err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, path, candidate[1:]...)
		cmd.Stdin = strings.NewReader(value)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return errors.New("no working system clipboard command found (install wl-clipboard or xclip on Linux)")
}
