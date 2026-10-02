package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Lotti/sogark/internal/terminal"
	"golang.org/x/term"
)

type prompter interface {
	Prompt(label, defaultVal string) (string, error)
	Close() error
}

func newPrompter(stdin io.Reader, stdout io.Writer) (prompter, error) {
	input, inputFile := stdin.(*os.File)
	output, outputFile := stdout.(*os.File)
	if inputFile && outputFile && input != nil && output != nil && term.IsTerminal(int(input.Fd())) && term.IsTerminal(int(output.Fd())) {
		return newTerminalPrompter(input, output)
	}

	return newFallbackPrompter(stdin, stdout), nil
}

type fallbackPrompter struct {
	reader *bufio.Reader
	writer io.Writer
}

func newFallbackPrompter(reader io.Reader, writer io.Writer) *fallbackPrompter {
	if reader == nil {
		reader = strings.NewReader("")
	}
	if writer == nil {
		writer = io.Discard
	}

	return &fallbackPrompter{
		reader: bufio.NewReader(reader),
		writer: writer,
	}
}

func (p *fallbackPrompter) Prompt(label, defaultVal string) (string, error) {
	defaultVal = strings.TrimSpace(defaultVal)
	if _, err := fmt.Fprint(p.writer, formatPrompt(label, defaultVal)); err != nil {
		return "", err
	}

	input, err := p.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	if err == io.EOF && strings.TrimSpace(input) == "" {
		return "", io.EOF
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal, nil
	}

	return input, nil
}

func (p *fallbackPrompter) Close() error {
	return nil
}

type terminalPrompter struct {
	fd            int
	output        *os.File
	state         *term.State
	term          *term.Terminal
	restoreOutput func() error
}

func newTerminalPrompter(stdin, stdout *os.File) (*terminalPrompter, error) {
	restore, err := terminal.EnableANSI(stdout)
	if err != nil {
		return nil, err
	}
	width, height, err := term.GetSize(int(stdout.Fd()))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("read wizard terminal size: %w", err), restore())
	}
	fd := int(stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, errors.Join(err, restore())
	}

	p := &terminalPrompter{
		fd: fd, output: stdout, state: state, restoreOutput: restore,
		term: term.NewTerminal(&terminalReadWriter{
			reader: stdin,
			writer: stdout,
		}, ""),
	}
	if err := p.term.SetSize(width, height); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	return p, nil
}

func (p *terminalPrompter) Prompt(label, defaultVal string) (string, error) {
	defaultVal = strings.TrimSpace(defaultVal)
	if p.output != nil {
		width, height, err := term.GetSize(int(p.output.Fd()))
		if err != nil {
			return "", fmt.Errorf("read wizard terminal size: %w", err)
		}
		if err := p.term.SetSize(width, height); err != nil {
			return "", err
		}
	}
	p.term.SetPrompt(formatPrompt(label, defaultVal))

	input, err := p.term.ReadLine()
	if err != nil && !errors.Is(err, term.ErrPasteIndicator) {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal, nil
	}

	return input, nil
}

func (p *terminalPrompter) Close() error {
	if p.state == nil {
		return nil
	}
	err := errors.Join(term.Restore(p.fd, p.state), p.restoreOutput())
	p.state = nil
	return err
}

type terminalReadWriter struct {
	reader io.Reader
	writer io.Writer
}

func (rw *terminalReadWriter) Read(b []byte) (int, error) {
	return rw.reader.Read(b)
}

func (rw *terminalReadWriter) Write(b []byte) (int, error) {
	return rw.writer.Write(b)
}

func formatPrompt(label, defaultVal string) string {
	if defaultVal != "" {
		return fmt.Sprintf("%s [%s]: ", label, defaultVal)
	}

	return fmt.Sprintf("%s: ", label)
}
