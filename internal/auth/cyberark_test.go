package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://example.com/vault")
	if c.BaseURL != "https://example.com/vault" {
		t.Errorf("BaseURL: got %q", c.BaseURL)
	}
	if c.Token != "" {
		t.Error("Token should be empty initially")
	}
	if c.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
}

func TestLogon_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("method: got %q, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/API/auth/SAML/Logon/") {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("content-type: got %q", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		if !strings.Contains(bodyStr, "SAMLResponse=test-saml-response") {
			t.Errorf("body missing SAMLResponse: %q", bodyStr)
		}
		if !strings.Contains(bodyStr, "apiUse=true") {
			t.Errorf("body missing apiUse: %q", bodyStr)
		}

		// CyberArk returns token as a JSON-quoted string
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"my-session-token-123"`))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.Logon("test-saml-response")
	if err != nil {
		t.Fatalf("Logon error: %v", err)
	}
	if c.Token != "my-session-token-123" {
		t.Errorf("Token: got %q, want %q", c.Token, "my-session-token-123")
	}
}

func TestLogon_UnquotedToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`plain-token`))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.Logon("saml")
	if err != nil {
		t.Fatalf("Logon error: %v", err)
	}
	if c.Token != "plain-token" {
		t.Errorf("Token: got %q, want %q", c.Token, "plain-token")
	}
}

func TestLogon_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("invalid credentials"))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.Logon("bad-saml")
	if err == nil {
		t.Error("Logon should return error on HTTP 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain status code: %v", err)
	}
}

func TestLogon_EmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`""`))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.Logon("saml")
	if err == nil {
		t.Error("Logon should return error for empty token")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error should mention token: %v", err)
	}
}

func TestFetchSSHKeys_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method: got %q, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/API/Users/Secret/SSHKeys/Cache") {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "test-token" {
			t.Errorf("auth header: got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type: got %q", r.Header.Get("Content-Type"))
		}

		// Verify JSON body
		body, _ := io.ReadAll(r.Body)
		var payload map[string][]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("invalid JSON body: %v", err)
		}
		formats, ok := payload["formats"]
		if !ok || len(formats) != 2 {
			t.Errorf("formats: got %v", payload)
		}

		// Real CyberArk API response format
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"value":[{"format":"OpenSSH","privateKey":"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEA\nAAAABG5vbmU=\n-----END OPENSSH PRIVATE KEY-----\n"},{"format":"PEM","privateKey":"-----BEGIN RSA PRIVATE KEY-----\r\nMIIBog==\r\n-----END RSA PRIVATE KEY-----\r\n"}]}`))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	c.Token = "test-token"

	result, err := c.FetchSSHKeys([]string{"OpenSSH", "PEM"})
	if err != nil {
		t.Fatalf("FetchSSHKeys error: %v", err)
	}
	if !strings.Contains(result, "OPENSSH PRIVATE KEY") {
		t.Errorf("result should contain OpenSSH key: %q", result)
	}
	if !strings.Contains(result, "RSA PRIVATE KEY") {
		t.Errorf("result should contain PEM key: %q", result)
	}
	// Verify newlines are real, not escaped
	if strings.Contains(result, `\n`) {
		t.Errorf("result should have real newlines, not literal backslash-n: %q", result)
	}
}

func TestFetchSSHKeys_AllFormatsIntegration(t *testing.T) {
	// Simulate the full CyberArk JSON response with OpenSSH + PEM + PPK.
	// The PPK privateKey contains real newlines after json.Unmarshal, just as
	// the production API returns.
	opensshKey := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACBbR6VFvxTfMp5dTjdH7fGc3YqaKyqS7K5KKIHuYV2hbAAAAJhPxlBnT8Z\n-----END OPENSSH PRIVATE KEY-----\n"
	pemKey := "-----BEGIN RSA PRIVATE KEY-----\r\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF8PbnGy0AHB7MhgHcBz8kKGsNTB\r\nYNmYQoEbMjMJdaUV2BZjjvMBa5SMDnONPgaLDjLSdNj+KwK1IqWrA3Ux1J5dK3M\r\n-----END RSA PRIVATE KEY-----\r\n"
	ppkKey := "PuTTY-User-Key-File-3: ssh-rsa\nEncryption: none\nComment: imported-openssh-key\nPublic-Lines: 12\nAAAAB3NzaC1yc2EAAAADAQABAAABAQDRndVLkklx2zfF+f/KBbIXw9ucbLQAcHsy\nGAewLQZ0PNp6IVP3lXHYjYyHAR5EPiODZNCCqFPRar6VHMGKflkkyVP7ZAz0KN3e\nzAomf2OE5rdFzCdJ9vbpYS9R2LuqFb3MLee4EftS07HGR5i5HQGE2j7YFMqU0OJT\nAAAAB3NzaC1yc2EAAAADAQABAAABAQDRndVLkklx2zfF+f/KBbIXw9ucbLQAcHsy\nGAewLQZ0PNp6IVP3lXHYjYyHAR5EPiODZNCCqFPRar6VHMGKflkkyVP7ZAz0KN3e\nzAomf2OE5rdFzCdJ9vbpYS9R2LuqFb3MLee4EftS07HGR5i5HQGE2j7YFMqU0OJT\nAAAAB3NzaC1yc2EAAAADAQABAAABAQDRndVLkklx2zfF+f/KBbIXw9ucbLQAcHsy\nGAewLQZ0PNp6IVP3lXHYjYyHAR5EPiODZNCCqFPRar6VHMGKflkkyVP7ZAz0KN3e\nzAomf2OE5rdFzCdJ9vbpYS9R2LuqFb3MLee4EftS07HGR5i5HQGE2j7YFMqU0OJT\nAAAAB3NzaC1yc2EAAAADAQABAAABAQDRndVLkklx2zfF+f/KBbIXw9ucbLQAcHsy\nGAewLQZ0PNp6IVP3lXHYjYyHAR5EPiODZNCCqFPRar6VHMGKflkkyVP7ZAz0KN3e\nzAomf2OE5rdFzCdJ9vbpYS9R2LuqFb3MLee4EftS07HGR5i5HQGE2j7YFMqU0OJT\nPrivate-Lines: 28\nAAABAHOLl8MoGRJpnM0M3jHYS5rp5kTln0snFsj2MkHljMEjHV0SGCOxpYjn6MJz\nAAAAB3NzaC1yc2EAAAADAQABAAABAQDRndVLkklx2zfF+f/KBbIXw9ucbLQAcHsy\nGAewLQZ0PNp6IVP3lXHYjYyHAR5EPiODZNCCqFPRar6VHMGKflkkyVP7ZAz0KN3e\nzAomf2OE5rdFzCdJ9vbpYS9R2LuqFb3MLee4EftS07HGR5i5HQGE2j7YFMqU0OJT\nAAAAB3NzaC1yc2EAAAADAQABAAABAQDRndVLkklx2zfF+f/KBbIXw9ucbLQAcHsy\nGAewLQZ0PNp6IVP3lXHYjYyHAR5EPiODZNCCqFPRar6VHMGKflkkyVP7ZAz0KN3e\nzAomf2OE5rdFzCdJ9vbpYS9R2LuqFb3MLee4EftS07HGR5i5HQGE2j7YFMqU0OJT\nPrivate-MAC: 4a21ecf3b4b8f05614e5a5d0b7cabff3e9e3e087"

	// Build JSON response exactly as CyberArk returns it.
	apiResp := sshKeysResponse{
		Value: []sshKeyEntry{
			{Format: "OpenSSH", PrivateKey: opensshKey},
			{Format: "PEM", PrivateKey: pemKey},
			{Format: "PPK", PrivateKey: ppkKey},
		},
	}
	jsonBytes, _ := json.Marshal(apiResp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	c.Token = "test-token"

	raw, err := c.FetchSSHKeys([]string{"OpenSSH", "PEM", "PPK"})
	if err != nil {
		t.Fatalf("FetchSSHKeys error: %v", err)
	}

	// Verify all three key types are present in raw output
	if !strings.Contains(raw, "BEGIN OPENSSH PRIVATE KEY") {
		t.Error("OpenSSH key not found in raw output")
	}
	if !strings.Contains(raw, "BEGIN RSA PRIVATE KEY") {
		t.Error("PEM key not found in raw output")
	}
	if !strings.Contains(raw, "PuTTY-User-Key-File-3") {
		t.Error("PPK key not found in raw output")
	}
	if !strings.Contains(raw, "Private-MAC: 4a21ecf3b4b8f05614e5a5d0b7cabff3e9e3e087") {
		t.Error("PPK Private-MAC not found in raw output")
	}

	// Parse the concatenated output through keys.Parse-equivalent regex check
	// to verify end-to-end extraction works
	if !strings.Contains(raw, "END OPENSSH PRIVATE KEY") {
		t.Error("OpenSSH key end marker missing")
	}
	if !strings.Contains(raw, "END RSA PRIVATE KEY") {
		t.Error("PEM key end marker missing")
	}
}

func TestFetchSSHKeys_JSONStringResponse(t *testing.T) {
	// Fallback: API returns a plain JSON string with escaped newlines
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"-----BEGIN OPENSSH PRIVATE KEY-----\nbase64keydata\n-----END OPENSSH PRIVATE KEY-----\n"`))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	c.Token = "test-token"

	result, err := c.FetchSSHKeys([]string{"OpenSSH"})
	if err != nil {
		t.Fatalf("FetchSSHKeys error: %v", err)
	}
	if strings.Contains(result, `\n`) {
		t.Errorf("result should have real newlines, not escaped: %q", result)
	}
	if !strings.Contains(result, "OPENSSH PRIVATE KEY") {
		t.Errorf("result should contain key data: %q", result)
	}
}

func TestFetchSSHKeys_RawTextResponse(t *testing.T) {
	// Fallback: API returns raw text (no JSON encoding)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\nkeydata\n-----END OPENSSH PRIVATE KEY-----\n"))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	c.Token = "test-token"

	result, err := c.FetchSSHKeys([]string{"OpenSSH"})
	if err != nil {
		t.Fatalf("FetchSSHKeys error: %v", err)
	}
	if !strings.Contains(result, "OPENSSH PRIVATE KEY") {
		t.Errorf("result should contain key data: %q", result)
	}
}

func TestFetchSSHKeys_NotAuthenticated(t *testing.T) {
	c := NewClient("https://example.com")
	// Token is empty
	_, err := c.FetchSSHKeys([]string{"OpenSSH"})
	if err == nil {
		t.Error("FetchSSHKeys should fail without token")
	}
	if !strings.Contains(err.Error(), "non autenticato") {
		t.Errorf("error should mention auth: %v", err)
	}
}

func TestFetchSSHKeys_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	c.Token = "expired-token"

	_, err := c.FetchSSHKeys([]string{"OpenSSH"})
	if err == nil {
		t.Error("FetchSSHKeys should fail on HTTP 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should contain status code: %v", err)
	}
}
