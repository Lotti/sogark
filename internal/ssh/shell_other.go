//go:build !windows

package ssh

import (
	"io"
	"os"
	"os/exec"
)

func startShell(child *exec.Cmd) (io.Reader, io.WriteCloser, func() error, func(), error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	child.Stdout, child.Stderr = writer, writer
	stdin, err := child.StdinPipe()
	if err != nil {
		reader.Close()
		writer.Close()
		return nil, nil, nil, nil, err
	}
	if err := child.Start(); err != nil {
		reader.Close()
		writer.Close()
		stdin.Close()
		return nil, nil, nil, nil, err
	}
	writer.Close()
	return reader, stdin, child.Wait, func() {
		stdin.Close()
		reader.Close()
		if child.Process != nil {
			_ = child.Process.Kill()
		}
	}, nil
}
