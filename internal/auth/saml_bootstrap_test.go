package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSAMLAcceptsTheReferencesUnquotedBootstrapURL(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/vault/bootstrap":
			fmt.Fprint(w, server.URL+"/identity/assertion")
		case "/identity/assertion":
			fmt.Fprint(w, `<input name="SAMLResponse" value="assertion">`)
		case "/vault/logon":
			_ = r.ParseForm()
			if r.PostForm.Get("SAMLResponse") != "assertion" {
				t.Error("the assertion was not submitted")
			}
			fmt.Fprint(w, `"pvwa-token"`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, _ := NewNativeClient(nativeProfile(server.URL, "saml"))
	client.authenticated = true
	if err := client.EstablishPVWA(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSAMLRejectsExternalBootstrapHTMLFormWithoutSendingIt(t *testing.T) {
	var externalRequests atomic.Int32
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		externalRequests.Add(1)
		t.Error("bootstrap form must never be sent to an unconfigured origin")
	}))
	defer external.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vault/bootstrap" {
			t.Error("unexpected request after intercepted bootstrap")
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body><form action="%s/auth?state=PRIVATE-STATE" method="post">
			<input name="SAMLResponse" value="PRIVATE-FORM-DATA"></form></body></html>`, external.URL)
	}))
	defer server.Close()
	client, _ := NewNativeClient(nativeProfile(server.URL, "saml"))
	client.authenticated = true
	err := client.EstablishPVWA(context.Background())
	if !errors.Is(err, errExternalRedirect) || !strings.Contains(err.Error(), "HTML form") ||
		strings.Contains(err.Error(), "PRIVATE") || strings.Contains(err.Error(), external.URL) {
		t.Fatalf("expected a redacted access-gateway error, got %v", err)
	}
	if externalRequests.Load() != 0 || client.credential != "" {
		t.Fatal("intercepted bootstrap must not establish a session or send the form")
	}
}

func TestSAMLBootstrapHTMLIsNotMistakenForALoginURLOrAssertion(t *testing.T) {
	client, _ := NewNativeClient(nativeProfile("https://vault.example.com", "saml"))
	for _, response := range []string{
		`<html>PRIVATE-BODY</html>`,
		`<form action="/vault/logon"><input name="SAMLResponse" value="PRIVATE-ASSERTION"></form>`,
		`<form action="https://vault.example.com/vault/logon"><input name="SAMLResponse" value="PRIVATE-ASSERTION"></form>`,
	} {
		target, err := client.samlBootstrapURL([]byte(response))
		if target != nil || err == nil || !strings.Contains(err.Error(), "HTML page") || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatalf("expected a redacted unexpected HTML error, got target=%v, error=%v", target, err)
		}
	}
}
