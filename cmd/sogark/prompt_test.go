package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"golang.org/x/term"
)

func TestTerminalPromptEditing(t *testing.T) {
	for _, test := range []struct {
		name, input, current, want string
	}{
		{"backspace", "abX\bcd\r", "", "abcd"},
		{"delete-backspace", "abX\x7fcd\r", "", "abcd"},
		{"cursor-delete", "abXcd\x1b[D\x1b[D\x1b[D\x1b[3~\r", "", "abcd"},
		{"trim-edges", "  user@example.com  \r", "", "user@example.com"},
		{"preserve-internal-spaces", " C:\\Program Files\\Tool \r", "", "C:\\Program Files\\Tool"},
		{"trim-default", "\r", " current ", "current"},
		{"unicode", "utèX\bnte\r", "", "utènte"},
		{"paste", "\x1b[200~ value \x1b[201~\r", "", "value"},
		{"crlf", "first\r\nsecond\r\n", "", "first"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			editor := term.NewTerminal(&terminalReadWriter{reader: strings.NewReader(test.input), writer: &output}, "")
			prompt := &terminalPrompter{term: editor}
			got, err := prompt.Prompt("Value", test.current)
			if err != nil || got != test.want {
				t.Fatalf("edited input = %q, err=%v; want %q", got, err, test.want)
			}
			if test.name == "crlf" {
				if next, err := prompt.Prompt("Next", ""); err != nil || next != "second" {
					t.Fatalf("CRLF produced a phantom empty answer: %q, %v", next, err)
				}
			}
		})
	}
}

func TestPromptCancellationAndWhitespace(t *testing.T) {
	for _, input := range []string{"", " "} {
		prompt := newFallbackPrompter(strings.NewReader(input), io.Discard)
		if _, err := prompt.Prompt("Value", "default"); !errors.Is(err, io.EOF) {
			t.Fatalf("EOF must not accept default: %v", err)
		}
	}
	for _, input := range []string{"\x03", "\x04"} {
		editor := term.NewTerminal(&terminalReadWriter{reader: strings.NewReader(input), writer: io.Discard}, "")
		if _, err := (&terminalPrompter{term: editor}).Prompt("Value", "default"); !errors.Is(err, io.EOF) {
			t.Fatalf("Ctrl+C/Ctrl+D must stop the wizard: %v", err)
		}
	}
	prompt := newFallbackPrompter(strings.NewReader("  \r\n C:\\Program Files\\Tool \r\n"), io.Discard)
	if got, err := prompt.Prompt("Default", " value "); err != nil || got != "value" {
		t.Fatalf("default whitespace was not removed: %q, %v", got, err)
	}
	if got, err := prompt.Prompt("Path", ""); err != nil || got != `C:\Program Files\Tool` {
		t.Fatalf("path whitespace handling failed: %q, %v", got, err)
	}
}
