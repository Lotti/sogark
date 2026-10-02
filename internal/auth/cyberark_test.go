package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKeyCacheRejectsNonKeyAndPartialResponses(t *testing.T) {
	for _, body := range []string{
		`<html>access gateway</html>`,
		`{}`,
		`{"value":[{"format":"OpenSSH","privateKey":""}]}`,
		`{"value":[{"format":"PEM","privateKey":"other"}]}`,
		`{"value":[{"format":"OpenSSH","privateKey":"one"},{"format":"OpenSSH","privateKey":"two"}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			client, _ := NewNativeClient(nativeProfile(server.URL, "saml"))
			client.credential = "token"
			if _, err := client.FetchSSHKeys(context.Background(), []string{"OpenSSH"}); err == nil {
				t.Fatal("must not accept HTTP 200 as successful key download")
			}
		})
	}
}

func TestKeyCacheBoundsResponsesAndRedactsErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat("private-key-data", maxResponseBytes/16+100))
	}))
	defer server.Close()
	client, _ := NewNativeClient(nativeProfile(server.URL, "saml"))
	client.credential = "token"
	_, err := client.FetchSSHKeys(context.Background(), []string{"OpenSSH"})
	if err == nil || strings.Contains(err.Error(), "private-key-data") {
		t.Fatalf("expected a bounded redacted response error, got %v", err)
	}
}
