package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func hiddenInputs(data []byte) (map[string]string, error) {
	values := make(map[string]string)
	tokenizer := html.NewTokenizer(bytes.NewReader(data))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			if err := tokenizer.Err(); err != io.EOF {
				return nil, errors.New("cannot parse authentication HTML")
			}
			return values, nil
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data != "input" {
				continue
			}
			var name, value string
			for _, attr := range token.Attr {
				switch attr.Key {
				case "name":
					name = attr.Val
				case "value":
					value = attr.Val
				}
			}
			if name == "SAMLResponse" || name == "code" || name == "state" {
				if _, exists := values[name]; exists {
					return nil, errors.New("duplicate authentication form input")
				}
				values[name] = value
			}
		}
	}
}

func (c *NativeClient) formRequest(ctx context.Context, phase, endpoint string, form url.Values) ([]byte, error) {
	data, _, err := c.request(ctx, phase, http.MethodPost, endpoint, "application/x-www-form-urlencoded", []byte(form.Encode()), nil, true)
	return data, err
}

func (c *NativeClient) EstablishPVWA(ctx context.Context) error {
	c.credential = ""
	if !c.authenticated {
		return errors.New("complete Identity QR/push authentication before PVWA logon")
	}
	switch c.profile.AuthType {
	case "saml":
		return c.establishSAML(ctx)
	case "oidc":
		return c.establishOIDC(ctx)
	default:
		return errors.New("unsupported PVWA authentication type")
	}
}

func (c *NativeClient) samlBootstrapURL(data []byte) (*url.URL, error) {
	var location string
	if err := json.Unmarshal(data, &location); err != nil {
		location = strings.TrimSpace(string(data))
	}
	target, err := url.Parse(location)
	if err == nil && target.IsAbs() {
		if !c.allowedURL(target) {
			return nil, fmt.Errorf("PVWA SAML login URL: %w", errExternalRedirect)
		}
		return target, nil
	}

	tokenizer := html.NewTokenizer(bytes.NewReader(data))
	isHTML := false
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			break
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		isHTML = true
		token := tokenizer.Token()
		if token.Data != "form" {
			continue
		}
		for _, attr := range token.Attr {
			if attr.Key != "action" || attr.Val == "" {
				continue
			}
			target, err := c.pvwaURL.Parse(attr.Val)
			if err == nil && !c.allowedURL(target) {
				return nil, fmt.Errorf("PVWA SAML bootstrap returned an HTML form: %w", errExternalRedirect)
			}
		}
	}
	if isHTML {
		return nil, errors.New("PVWA SAML bootstrap returned an HTML page instead of a login URL; check the configured PVWA endpoint and network/access-gateway session")
	}
	return nil, errors.New("PVWA SAML bootstrap did not return a login URL")
}

func (c *NativeClient) establishSAML(ctx context.Context) error {
	form := url.Values{"apiUse": {"true"}, "concurrentSession": {"true"}, "SAMLResponse": {""}}
	data, err := c.formRequest(ctx, "PVWA SAML bootstrap", c.profile.SAMLBootstrapURL, form)
	if err != nil {
		return err
	}
	target, err := c.samlBootstrapURL(data)
	if err != nil {
		return err
	}
	data, _, err = c.request(ctx, "PVWA SAML assertion", http.MethodGet, target.String(), "", nil, nil, true)
	if err != nil {
		return err
	}
	values, err := hiddenInputs(data)
	if err != nil {
		return err
	}
	if strings.TrimSpace(values["SAMLResponse"]) == "" {
		return errors.New("PVWA SAML assertion not found in the returned form")
	}
	form.Set("SAMLResponse", values["SAMLResponse"])
	data, err = c.formRequest(ctx, "PVWA SAML logon", c.profile.SAMLLogonURL, form)
	if err != nil {
		return err
	}
	var token string
	if err := json.Unmarshal(data, &token); err != nil || strings.TrimSpace(token) == "" {
		return errors.New("PVWA SAML logon did not return a session token")
	}
	c.credential = token
	return nil
}

func (c *NativeClient) establishOIDC(ctx context.Context) error {
	c.httpClient.Jar.SetCookies(c.pvwaURL, []*http.Cookie{{
		Name: "PreferredCulture", Value: "en",
		Path:   strings.TrimRight(c.pvwaURL.Path, "/") + "/",
		Secure: c.pvwaURL.Scheme == "https",
	}})
	_, location, err := c.request(ctx, "PVWA OIDC authorize", http.MethodGet, c.profile.OIDCAuthorizeURL, "", nil, nil, true)
	if err != nil {
		return err
	}
	data, _, err := c.request(ctx, "PVWA OIDC form", http.MethodGet, location.String(), "", nil, nil, true)
	if err != nil {
		return err
	}
	values, err := hiddenInputs(data)
	if err != nil {
		return err
	}
	if strings.TrimSpace(values["code"]) == "" || strings.TrimSpace(values["state"]) == "" {
		return errors.New("PVWA OIDC code/state not found in the returned form")
	}
	_, err = c.formRequest(ctx, "PVWA OIDC token", c.profile.OIDCTokenURL, url.Values{
		"code": {values["code"]}, "state": {values["state"]},
	})
	if err != nil {
		return err
	}
	cacheURL, _ := url.Parse(c.profile.SSHKeysCacheURL)
	for _, cookie := range c.httpClient.Jar.Cookies(cacheURL) {
		if cookie.Name == "CA66666" && cookie.Value != "" {
			c.credential = cookie.Value
			return nil
		}
	}
	return errors.New("PVWA OIDC token response did not establish the cache session cookie")
}
