package main

import (
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	input := "abc123  sogark-linux-amd64\nDEF456 *sogark-windows-amd64.exe\n"

	got, err := parseChecksums(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseChecksums returned error: %v", err)
	}

	if got["sogark-linux-amd64"] != "abc123" {
		t.Fatalf("linux checksum = %q, want %q", got["sogark-linux-amd64"], "abc123")
	}

	if got["sogark-windows-amd64.exe"] != "def456" {
		t.Fatalf("windows checksum = %q, want %q", got["sogark-windows-amd64.exe"], "def456")
	}
}
