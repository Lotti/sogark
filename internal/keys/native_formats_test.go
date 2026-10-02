package keys

import (
	"strings"
	"testing"
)

func TestNativePEMFormatsAndMatchingMarkers(t *testing.T) {
	for _, marker := range []string{"PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "DSA PRIVATE KEY"} {
		key := "-----BEGIN " + marker + "-----\nfixture\n-----END " + marker + "-----"
		parsed, err := Parse(key)
		if err != nil || parsed.PEM == "" {
			t.Fatalf("missing supported PEM format %s: %v", marker, err)
		}
		if err := parsed.RequireFormats([]string{"PEM"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Parse("-----BEGIN RSA PRIVATE KEY-----\nfixture\n-----END EC PRIVATE KEY-----"); err == nil {
		t.Fatal("mismatched PEM markers must be rejected")
	}
}

func TestRequireAllRequestedFormatsBeforeSaving(t *testing.T) {
	parsed, err := Parse("-----BEGIN OPENSSH PRIVATE KEY-----\nfixture\n-----END OPENSSH PRIVATE KEY-----")
	if err != nil {
		t.Fatal(err)
	}
	if err := parsed.RequireFormats([]string{"OpenSSH", "PEM"}); err == nil || !strings.Contains(err.Error(), "PEM") {
		t.Fatal("missing requested key must be an error before storage")
	}
}
