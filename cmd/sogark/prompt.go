package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type prompter interface {
	Prompt(label, defaultVal string) (string, error)
	Close() error
}

func newPrompter(stdin, stdout *os.File) prompter {
	if stdin != nil && stdout != nil && term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd())) {
		if tp, err := newTerminalPrompter(stdin, stdout); err == nil {
			return tp
		}
	}

	return newFallbackPrompter(stdin, stdout)
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
	if _, err := fmt.Fprint(p.writer, formatPrompt(label, defaultVal)); err != nil {
		return "", err
	}

	input, err := p.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
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
	fd    int
	state *term.State
	term  *term.Terminal
}

func newTerminalPrompter(stdin, stdout *os.File) (*terminalPrompter, error) {
	fd := int(stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}

	return &terminalPrompter{
		fd:    fd,
		state: state,
		term: term.NewTerminal(&terminalReadWriter{
			reader: stdin,
			writer: stdout,
		}, ""),
	}, nil
}

func (p *terminalPrompter) Prompt(label, defaultVal string) (string, error) {
	p.term.SetPrompt(formatPrompt(label, defaultVal))

	input, err := p.term.ReadLine()
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal, nil
	}

	return input, nil
}

func (p *terminalPrompter) Close() error {
	return term.Restore(p.fd, p.state)
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
