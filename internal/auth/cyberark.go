package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Lotti/sogark/internal/config"
)

type sshKeyEntry struct {
	Format     string `json:"format"`
	PrivateKey string `json:"privateKey"`
}

type sshKeysResponse struct {
	Value []sshKeyEntry `json:"value"`
}

type pvwaAccount struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	User    string `json:"userName"`
}

type pvwaAccountsResponse struct {
	Value    []pvwaAccount `json:"value"`
	NextLink string        `json:"nextLink"`
}

// FindAccountID matches the Ansible CCP lookup fields against PVWA account
// metadata: HostName maps to address and UserName maps to userName. Safe=.*
// means the search spans every Safe visible to the authenticated user.
func (c *NativeClient) FindAccountID(ctx context.Context, hostname, username string) (string, error) {
	if c.credential == "" {
		return "", errors.New("PVWA session is not authenticated")
	}
	hostname = normalizeHost(hostname)
	username = strings.TrimSpace(username)
	if hostname == "" || username == "" {
		return "", errors.New("machine name and target username are required")
	}
	endpoint, err := url.Parse(strings.TrimRight(c.profile.PVWABaseURL, "/") + "/api/Accounts")
	if err != nil {
		return "", errors.New("invalid PVWA accounts endpoint")
	}
	query := endpoint.Query()
	query.Set("search", shortHost(hostname))
	query.Set("searchType", "contains")
	query.Set("limit", "1000")
	endpoint.RawQuery = query.Encode()
	data, _, err := c.request(ctx, "PVWA account search", http.MethodGet, endpoint.String(), "", nil, c.sessionHeaders(), false)
	if err != nil {
		return "", err
	}
	var response pvwaAccountsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return "", errors.New("PVWA account search returned an unexpected response")
	}
	if response.NextLink != "" {
		return "", errors.New("PVWA account search returned more than 1000 results; search using the machine FQDN")
	}
	var matches []pvwaAccount
	for _, account := range response.Value {
		if hostMatches(normalizeHost(account.Address), hostname) && strings.EqualFold(strings.TrimSpace(account.User), username) {
			matches = append(matches, account)
		}
	}
	switch len(matches) {
	case 0:
		return "", errors.New("no CyberArk account matched this machine and target username")
	case 1:
		if strings.TrimSpace(matches[0].ID) == "" {
			return "", errors.New("PVWA account search returned an account without an ID")
		}
		return matches[0].ID, nil
	default:
		return "", errors.New("multiple CyberArk accounts matched; specify a fully qualified machine name or a different target username")
	}
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func shortHost(host string) string {
	host = normalizeHost(host)
	if dot := strings.IndexByte(host, '.'); dot >= 0 {
		return host[:dot]
	}
	return host
}

func hostMatches(address, hostname string) bool {
	if address == hostname {
		return true
	}
	if strings.Contains(hostname, ".") {
		// Ansible's query checks inventory_hostname_short and inventory_hostname.
		// For an FQDN input, the short form is also an exact candidate.
		return address == shortHost(hostname)
	}
	// A short input can match its exact short form or an FQDN with that
	// exact first DNS label. Multiple domains are rejected as ambiguous.
	return strings.HasPrefix(address, hostname+".")
}

func (c *NativeClient) sessionHeaders() http.Header {
	headers := make(http.Header)
	switch c.profile.AuthType {
	case "saml":
		headers.Set("Authorization", c.credential)
	case "oidc":
		headers.Set("x-ca66666", c.credential)
		headers.Set("Accept", "application/json; flat=true; version=1.0")
	}
	return headers
}

// RetrieveAccountPassword requests a credential from the PVWA Accounts API.
// The API is intended for interactive, human use and may enforce a reason or ticket.
func (c *NativeClient) RetrieveAccountPassword(ctx context.Context, accountID, reason, ticketID string) (string, error) {
	if c.credential == "" {
		return "", errors.New("PVWA session is not authenticated")
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" || strings.ContainsAny(accountID, "/?#") {
		return "", errors.New("a valid CyberArk account ID is required")
	}
	endpoint := strings.TrimRight(c.profile.PVWABaseURL, "/") + "/api/Accounts/" + url.PathEscape(accountID) + "/Password/Retrieve"
	body, err := json.Marshal(map[string]any{
		"reason": reason, "TicketId": ticketID, "ActionType": "copy", "isUse": true,
	})
	if err != nil {
		return "", errors.New("cannot serialize password retrieval request")
	}
	data, _, err := c.request(ctx, "PVWA account password retrieval", http.MethodPost, endpoint, "application/json", body, c.sessionHeaders(), false)
	if err != nil {
		return "", err
	}
	// PVWA returns the secret as text. Accept a JSON string response as well,
	// while keeping malformed or empty responses out of terminal errors.
	password := string(data)
	var decoded string
	if json.Unmarshal(data, &decoded) == nil {
		password = decoded
	}
	if strings.TrimSpace(password) == "" {
		return "", errors.New("PVWA returned an empty account password")
	}
	return password, nil
}

func (c *NativeClient) FetchSSHKeys(ctx context.Context, formats []string) (string, error) {
	if c.credential == "" {
		return "", errors.New("PVWA session is not authenticated")
	}
	formats, err := config.NormalizeKeyFormats(formats)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string][]string{"formats": formats})
	if err != nil {
		return "", errors.New("cannot serialize key formats")
	}
	data, _, err := c.request(ctx, "PVWA SSH key cache", http.MethodPost, c.profile.SSHKeysCacheURL, "application/json", body, c.sessionHeaders(), false)
	if err != nil {
		return "", err
	}
	var response sshKeysResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return "", errors.New("PVWA SSH key cache returned non-key JSON/HTML instead of value[].format/privateKey")
	}
	requested := make(map[string]bool, len(formats))
	for _, format := range formats {
		requested[format] = true
	}
	received := make(map[string]bool, len(formats))
	var parts []string
	for _, entry := range response.Value {
		if !requested[entry.Format] {
			continue
		}
		if received[entry.Format] || strings.TrimSpace(entry.PrivateKey) == "" {
			return "", errors.New("PVWA SSH key cache returned a duplicate or empty requested key")
		}
		received[entry.Format] = true
		parts = append(parts, entry.PrivateKey)
	}
	for _, format := range formats {
		if !received[format] {
			return "", fmt.Errorf("PVWA SSH key cache did not return the requested %s key", format)
		}
	}
	return strings.Join(parts, "\n"), nil
}
