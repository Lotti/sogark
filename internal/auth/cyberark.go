package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	headers := make(http.Header)
	switch c.profile.AuthType {
	case "saml":
		headers.Set("Authorization", c.credential)
	case "oidc":
		headers.Set("x-ca66666", c.credential)
		headers.Set("Accept", "application/json; flat=true; version=1.0")
	}
	data, _, err := c.request(ctx, "PVWA SSH key cache", http.MethodPost, c.profile.SSHKeysCacheURL, "application/json", body, headers, false)
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
