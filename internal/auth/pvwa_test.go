package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func samlForm(assertion string) url.Values {
	return url.Values{"apiUse": {"true"}, "concurrentSession": {"true"}, "SAMLResponse": {assertion}}
}

func TestPVWARequiresIdentitySession(t *testing.T) {
	client, err := NewNativeClient(nativeProfile("https://example.com", "saml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.EstablishPVWA(context.Background()); err == nil {
		t.Fatal("PVWA logon must require Identity success")
	}
	if _, err := client.FetchSSHKeys(context.Background(), []string{"OpenSSH"}); err == nil {
		t.Fatal("key cache must require a PVWA credential")
	}
}

func TestHiddenInputsDecodeHTMLWithoutAttributeOrderAssumptions(t *testing.T) {
	inputs, err := hiddenInputs([]byte(`<form><input VALUE="s&#x2b;a&#x3d;m" NAME="SAMLResponse">
		<input value="c&amp;o" name="code"><input type="hidden" name="state" value="s&amp;t"></form>`))
	if err != nil || inputs["SAMLResponse"] != "s+a=m" || inputs["code"] != "c&o" || inputs["state"] != "s&t" {
		t.Fatalf("unexpected decoded form: %v, err=%v", inputs, err)
	}
	if _, err := hiddenInputs([]byte(`<input name="code" value="one"><input name="code" value="two">`)); err == nil {
		t.Fatal("duplicate credentials must not be accepted")
	}
}

func TestPVWARejectsMissingOIDCFieldsAndCookie(t *testing.T) {
	for _, response := range []string{`<html>login required</html>`, `<input name="code" value="code"><input name="state" value="state">`} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, response)
			}))
			defer server.Close()
			client, _ := NewNativeClient(nativeProfile(server.URL, "oidc"))
			client.authenticated = true
			if err := client.EstablishPVWA(context.Background()); err == nil || client.credential != "" {
				t.Fatal("OIDC must reject a missing form or session cookie")
			}
		})
	}
}

func TestPVWAErrorBodyIsNotPrinted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "PRIVATE-TOKEN private-key-material", http.StatusUnauthorized)
	}))
	defer server.Close()
	client, _ := NewNativeClient(nativeProfile(server.URL, "saml"))
	client.authenticated = true
	err := client.EstablishPVWA(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "PRIVATE") {
		t.Fatalf("expected a redacted HTTP error, got %v", err)
	}
}
