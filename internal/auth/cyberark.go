package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Client handles communication with the CyberArk PVWA REST API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a new CyberArk API client.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
}

// Logon authenticates using a SAML response and stores the session token.
func (c *Client) Logon(samlResponse string) error {
	loginURL := c.BaseURL + "/API/auth/SAML/Logon/"

	form := url.Values{}
	form.Set("apiUse", "true")
	form.Set("concurrentSession", "true")
	form.Set("SAMLResponse", samlResponse)

	resp, err := c.HTTPClient.PostForm(loginURL, form)
	if err != nil {
		return fmt.Errorf("logon fallito: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("errore lettura risposta logon: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("logon fallito (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// The token is returned as a JSON string (quoted)
	token := string(body)
	// Remove surrounding quotes if present
	if len(token) >= 2 && token[0] == '"' && token[len(token)-1] == '"' {
		token = token[1 : len(token)-1]
	}

	if token == "" {
		return fmt.Errorf("token di sessione non ricevuto")
	}

	c.Token = token
	return nil
}

// sshKeyEntry represents a single key in the CyberArk API response.
type sshKeyEntry struct {
	Format     string `json:"format"`
	PrivateKey string `json:"privateKey"`
}

// sshKeysResponse represents the CyberArk SSHKeys/Cache API response.
type sshKeysResponse struct {
	Value []sshKeyEntry `json:"value"`
}

// FetchSSHKeys retrieves SSH keys from the MFA cache in the specified formats.
func (c *Client) FetchSSHKeys(formats []string) (string, error) {
	if c.Token == "" {
		return "", fmt.Errorf("non autenticato: esegui prima il login")
	}

	keysURL := c.BaseURL + "/API/Users/Secret/SSHKeys/Cache"

	payload := map[string][]string{"formats": formats}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("errore serializzazione richiesta: %w", err)
	}

	req, err := http.NewRequest("POST", keysURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("errore creazione richiesta: %w", err)
	}
	req.Header.Set("Authorization", c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch chiavi fallito: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("errore lettura risposta chiavi: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch chiavi fallito (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// The API returns JSON: {"value":[{"format":"OpenSSH","privateKey":"..."},...]}.
	// json.Unmarshal automatically unescapes \n and \r\n in privateKey strings.
	var parsed sshKeysResponse
	if err := json.Unmarshal(body, &parsed); err == nil && len(parsed.Value) > 0 {
		var parts []string
		for _, entry := range parsed.Value {
			if entry.PrivateKey != "" {
				parts = append(parts, entry.PrivateKey)
			}
		}
		raw := strings.Join(parts, "\n")

		if os.Getenv("SOGARK_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] FetchSSHKeys: parsed %d key(s) from JSON response\n", len(parts))
			for _, e := range parsed.Value {
				preview := e.PrivateKey
				if len(preview) > 80 {
					preview = preview[:80] + "..."
				}
				fmt.Fprintf(os.Stderr, "[DEBUG]   format=%s, len=%d, preview=%q\n", e.Format, len(e.PrivateKey), preview)
			}
		}

		return raw, nil
	}

	// Fallback: try as JSON string, then raw text
	raw := string(body)
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 && trimmed[0] == '"' {
		var unescaped string
		if err := json.Unmarshal([]byte(trimmed), &unescaped); err == nil {
			raw = unescaped
		}
	}

	if os.Getenv("SOGARK_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] FetchSSHKeys: fallback to raw text (%d bytes)\n", len(raw))
	}

	return raw, nil
}
