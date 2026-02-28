package keys

import (
	"strings"
	"testing"
)

const sampleOpenSSH = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBbR6VFvxTfMp5dTjdH7fGc3YqaKyqS7K5KKIHuYV2hbAAAAJhPxlBnT8Z
QZwAAAAtzc2gtZWQyNTUxOQAAACBbR6VFvxTfMp5dTjdH7fGc3YqaKyqS7K5KKIHuYV2h
bAAAAEC5h3p7nMDdm2P+gkXYg5mZPr2mFm0n3R2C6F8sGFjqltHpUW/FN8ynl1ON0ft8Z
-----END OPENSSH PRIVATE KEY-----`

const samplePEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF8PbnGy0AHB7MhgHcBz8kKGsNTB
YNmYQoEbMjMJdaUV2BZjjvMBa5SMDnONPgaLDjLSdNj+KwK1IqWrA3Ux1J5dK3M
zKfHI8ygLqz0zAomf2OE5rdFzCdJ9vbpYS9R2LuqFb3MLee4EftS07HGR5i5HQGE
-----END RSA PRIVATE KEY-----`

const samplePPK = `PuTTY-User-Key-File-3: ssh-rsa
Encryption: none
Comment: imported-openssh-key
Public-Lines: 6
AAAAB3NzaC1yc2EAAAADAQABAAABAQDRndVLkklx2zfF+f/KBbIXw9ucbLQAcHsy
Private-Lines: 14
AAABAHOLl8MoGRJpnM0M3jHYS5rp5kTln0snFsj2MkHljMEjHV0SGCOxpYjn6MJz
Private-MAC: 4a21ecf3b4b8f05614e5a5d0b7cabff3e9e3e087`

func TestParse_AllFormats(t *testing.T) {
	raw := "some preamble\n" + sampleOpenSSH + "\nmore text\n" + samplePEM + "\nand\n" + samplePPK + "\ntrailer"

	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if !strings.Contains(parsed.OpenSSH, "BEGIN OPENSSH PRIVATE KEY") {
		t.Error("OpenSSH key not found")
	}
	if !strings.Contains(parsed.OpenSSH, "END OPENSSH PRIVATE KEY") {
		t.Error("OpenSSH key end marker not found")
	}

	if !strings.Contains(parsed.PEM, "BEGIN RSA PRIVATE KEY") {
		t.Error("PEM key not found")
	}
	if !strings.Contains(parsed.PEM, "END RSA PRIVATE KEY") {
		t.Error("PEM key end marker not found")
	}

	if !strings.Contains(parsed.PPK, "PuTTY-User-Key-File-3") {
		t.Error("PPK key not found")
	}
	if !strings.Contains(parsed.PPK, "Private-MAC:") {
		t.Error("PPK Private-MAC not found")
	}
}

func TestParse_OpenSSHOnly(t *testing.T) {
	parsed, err := Parse(sampleOpenSSH)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.OpenSSH == "" {
		t.Error("OpenSSH key should not be empty")
	}
	if parsed.PEM != "" {
		t.Error("PEM key should be empty")
	}
	if parsed.PPK != "" {
		t.Error("PPK key should be empty")
	}
}

func TestParse_PEMOnly(t *testing.T) {
	parsed, err := Parse(samplePEM)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.PEM == "" {
		t.Error("PEM key should not be empty")
	}
	if parsed.OpenSSH != "" {
		t.Error("OpenSSH key should be empty")
	}
}

func TestParse_PPKOnly(t *testing.T) {
	parsed, err := Parse(samplePPK)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.PPK == "" {
		t.Error("PPK key should not be empty")
	}
	if parsed.OpenSSH != "" {
		t.Error("OpenSSH key should be empty")
	}
}

func TestParse_NoKeys(t *testing.T) {
	_, err := Parse("no keys here at all")
	if err == nil {
		t.Error("Parse should return error when no keys found")
	}
	if !strings.Contains(err.Error(), "nessun blocco chiave") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Error("Parse should return error on empty input")
	}
}

func TestNormalize_RemovesDuplicateEmptyLines(t *testing.T) {
	input := "line1\n\n\n\nline2\n\nline3"
	result := normalize(input)
	lines := strings.Split(strings.TrimSpace(result), "\n")

	// Should have: line1, "", line2, "", line3
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d: %v", len(lines), lines)
	}
}

func TestNormalize_TrimsTrailingWhitespace(t *testing.T) {
	input := "line1   \t\nline2\r\n"
	result := normalize(input)
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if strings.TrimRight(line, " \t\r") != line {
			t.Errorf("line %d has trailing whitespace: %q", i, line)
		}
	}
}

func TestNormalize_EndsWithNewline(t *testing.T) {
	result := normalize("hello")
	if !strings.HasSuffix(result, "\n") {
		t.Error("normalized output should end with newline")
	}
}
