package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIdentityRejectedResponseIsRedacted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":false,"Message":"PRIVATE-PASSWORD PRIVATE-TOKEN"}`)
	}))
	defer server.Close()
	client, _ := NewNativeClient(nativeProfile(server.URL, "saml"))
	err := client.AuthenticateIdentity(context.Background(), "user", &recordingQR{})
	if err == nil || strings.Contains(err.Error(), "PRIVATE") || client.authenticated {
		t.Fatalf("expected a redacted rejection error, got %v", err)
	}
}

func TestIdentityPushRefusalStopsAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := identityResult{Summary: "Failure"}
		if r.URL.Path == "/identity/start" {
			result = identityResult{SessionID: "session", Challenges: []identityChallenge{
				{Mechanisms: []identityMechanism{{Name: "QR", MechanismID: "qr", Image: "image"}}},
			}}
		}
		writeIdentity(t, w, result)
	}))
	defer server.Close()
	client, _ := NewNativeClient(nativeProfile(server.URL, "saml"))
	err := client.AuthenticateIdentity(context.Background(), "user", &recordingQR{})
	if err == nil || !strings.Contains(err.Error(), "rejected") || client.authenticated {
		t.Fatalf("expected an authentication failure, got %v", err)
	}
}
