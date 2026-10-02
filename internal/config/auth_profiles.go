package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type AuthProfile struct {
	AuthType                 string `yaml:"auth_type"`
	TenantID                 string `yaml:"tenant_id"`
	PVWABaseURL              string `yaml:"pvwa_base_url"`
	StartAuthenticationURL   string `yaml:"start_authentication_url"`
	AdvanceAuthenticationURL string `yaml:"advance_authentication_url"`
	SAMLBootstrapURL         string `yaml:"saml_bootstrap_url,omitempty"`
	SAMLLogonURL             string `yaml:"saml_logon_url,omitempty"`
	OIDCAuthorizeURL         string `yaml:"oidc_authorize_url,omitempty"`
	OIDCTokenURL             string `yaml:"oidc_token_url,omitempty"`
	SSHKeysCacheURL          string `yaml:"ssh_keys_cache_url"`
}

func (p *AuthProfile) Fields() map[string]*string {
	return map[string]*string{
		"auth_type": &p.AuthType, "tenant_id": &p.TenantID,
		"pvwa_base_url":              &p.PVWABaseURL,
		"start_authentication_url":   &p.StartAuthenticationURL,
		"advance_authentication_url": &p.AdvanceAuthenticationURL,
		"saml_bootstrap_url":         &p.SAMLBootstrapURL, "saml_logon_url": &p.SAMLLogonURL,
		"oidc_authorize_url": &p.OIDCAuthorizeURL, "oidc_token_url": &p.OIDCTokenURL,
		"ssh_keys_cache_url": &p.SSHKeysCacheURL,
	}
}

func (p AuthProfile) RequiredFields() []string {
	fields := []string{"tenant_id", "pvwa_base_url", "start_authentication_url", "advance_authentication_url", "ssh_keys_cache_url"}
	switch p.AuthType {
	case "saml":
		return append(fields, "saml_bootstrap_url", "saml_logon_url")
	case "oidc":
		return append(fields, "oidc_authorize_url", "oidc_token_url")
	default:
		return fields
	}
}

func (p AuthProfile) Validate() error {
	if p.AuthType != "saml" && p.AuthType != "oidc" {
		return errors.New("auth_type must be saml or oidc")
	}
	fields := p.Fields()
	for _, name := range p.RequiredFields() {
		if strings.TrimSpace(*fields[name]) == "" {
			return fmt.Errorf("%s is required", name)
		}
		if name == "tenant_id" {
			continue
		}
		if _, err := ParseEndpoint(*fields[name]); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	identity, _ := ParseEndpoint(p.StartAuthenticationURL)
	pvwa, _ := ParseEndpoint(p.PVWABaseURL)
	for _, name := range p.RequiredFields() {
		if name == "tenant_id" {
			continue
		}
		endpoint, _ := ParseEndpoint(*fields[name])
		expected := pvwa
		if name == "start_authentication_url" || name == "advance_authentication_url" {
			expected = identity
		}
		if !SameOrigin(endpoint, expected) {
			return fmt.Errorf("%s has a different origin from its configured service", name)
		}
	}
	return nil
}

func (c *Config) SelectedAuthProfile() (AuthProfile, error) {
	if !profileNamePattern.MatchString(c.AuthProfile) {
		return AuthProfile{}, errors.New("select a named auth_profile; legacy browser/Identity-only configuration must be migrated")
	}
	p, ok := c.AuthProfiles[c.AuthProfile]
	if !ok {
		return AuthProfile{}, errors.New("the selected auth_profile does not exist")
	}
	if err := p.Validate(); err != nil {
		return AuthProfile{}, fmt.Errorf("selected authentication profile: %w", err)
	}
	return p, nil
}

func ParseEndpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.TrimSpace(raw) != raw {
		return nil, errors.New("must be an absolute HTTPS URL without credentials, query or fragment")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return nil, errors.New("must use HTTPS")
	}
	return u, nil
}

func SameOrigin(a, b *url.URL) bool {
	return a.Scheme == b.Scheme && strings.EqualFold(a.Host, b.Host)
}
