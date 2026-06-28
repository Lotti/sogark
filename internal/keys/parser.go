package keys

import (
	"fmt"
	"regexp"
	"strings"

	msg "github.com/Lotti/sogark/internal/messages"
)

// Parsed holds the extracted key blocks from the raw API response.
type Parsed struct {
	OpenSSH string
	PEM     string
	PPK     string
}

var (
	opensshPattern = regexp.MustCompile(`(?ms)-----BEGIN OPENSSH PRIVATE KEY-----\s*.*?\s*-----END OPENSSH PRIVATE KEY-----`)
	pemPattern     = regexp.MustCompile(`(?ms)-----BEGIN RSA PRIVATE KEY-----\s*.*?\s*-----END RSA PRIVATE KEY-----`)
	ppkPattern     = regexp.MustCompile(`(?ms)PuTTY-User-Key-File-\d+:[^\r\n]*\r?\n.*?Private-MAC:\s*[0-9a-fA-F]+`)
)

// Parse extracts SSH key blocks from the raw CyberArk API response text.
func Parse(raw string) (*Parsed, error) {
	p := &Parsed{}

	if m := opensshPattern.FindString(raw); m != "" {
		p.OpenSSH = normalize(m)
	}
	if m := pemPattern.FindString(raw); m != "" {
		p.PEM = normalize(m)
	}
	if m := ppkPattern.FindString(raw); m != "" {
		p.PPK = normalize(m)
	}

	if p.OpenSSH == "" && p.PEM == "" && p.PPK == "" {
		return nil, fmt.Errorf(msg.KeysNoBlockFound)
	}

	return p, nil
}

// normalize trims whitespace and removes duplicate empty lines.
func normalize(text string) string {
	lines := strings.Split(text, "\n")
	var result []string
	prevEmpty := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		isEmpty := strings.TrimSpace(line) == ""
		if isEmpty {
			if !prevEmpty {
				result = append(result, "")
			}
			prevEmpty = true
		} else {
			result = append(result, line)
			prevEmpty = false
		}
	}
	return strings.TrimSpace(strings.Join(result, "\n")) + "\n"
}
