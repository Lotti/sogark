package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Lotti/sogark/internal/config"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/term"
)

const maxResponseBytes = 4 << 20

var errExternalRedirect = errors.New("redirect to an unconfigured origin; check access-gateway authentication and network/proxy configuration")

type QRPrompter interface {
	DisplayQR(string) error
	Printf(string, ...any)
}

type TerminalPrompter struct {
	in  *os.File
	out io.Writer
}

func NewTerminalPrompter(in *os.File, out io.Writer) (*TerminalPrompter, error) {
	if in == nil || !term.IsTerminal(int(in.Fd())) {
		return nil, errors.New("QR authentication requires an interactive terminal")
	}
	if out == nil {
		return nil, errors.New("QR authentication requires terminal output")
	}
	return &TerminalPrompter{in: in, out: out}, nil
}

func (p *TerminalPrompter) Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(p.out, format, args...)
}

type NativeClient struct {
	profile       config.AuthProfile
	httpClient    *http.Client
	identityURL   *url.URL
	pvwaURL       *url.URL
	pollInterval  time.Duration
	authenticated bool
	credential    string
}

func NewNativeClient(profile config.AuthProfile) (*NativeClient, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, fmt.Errorf("create authentication cookie jar: %w", err)
	}
	identity, _ := config.ParseEndpoint(profile.StartAuthenticationURL)
	pvwa, _ := config.ParseEndpoint(profile.PVWABaseURL)
	c := &NativeClient{
		profile: profile, identityURL: identity, pvwaURL: pvwa,
		pollInterval: 2 * time.Second,
	}
	c.httpClient = &http.Client{
		Jar: jar, Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many authentication redirects")
			}
			if !c.allowedURL(req.URL) {
				return errExternalRedirect
			}
			req.Header.Del("Authorization")
			req.Header.Del("x-ca66666")
			return nil
		},
	}
	return c, nil
}

func (c *NativeClient) allowedURL(u *url.URL) bool {
	return u.User == nil && u.Fragment == "" &&
		(config.SameOrigin(u, c.identityURL) || config.SameOrigin(u, c.pvwaURL))
}

func (c *NativeClient) request(ctx context.Context, phase, method, endpoint, contentType string, body []byte, headers http.Header, redirects bool) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: invalid request URL", phase)
	}
	if !c.allowedURL(req.URL) {
		return nil, nil, fmt.Errorf("%s: %w", phase, errExternalRedirect)
	}
	for name, values := range headers {
		req.Header[name] = append([]string(nil), values...)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := *c.httpClient
	if !redirects {
		client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
			if !c.allowedURL(req.URL) {
				return errExternalRedirect
			}
			return http.ErrUseLastResponse
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("%s: %w", phase, ctx.Err())
		}
		if errors.Is(err, errExternalRedirect) {
			return nil, nil, fmt.Errorf("%s: %w", phase, errExternalRedirect)
		}
		return nil, nil, fmt.Errorf("%s: network request failed; check TLS, connectivity and proxy settings", phase)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("%s: HTTP %d", phase, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("%s: %w", phase, ctx.Err())
		}
		return nil, nil, fmt.Errorf("%s: cannot read response", phase)
	}
	if len(data) > maxResponseBytes {
		return nil, nil, fmt.Errorf("%s: response exceeds size limit", phase)
	}
	return data, resp.Request.URL, nil
}

type identityResponse struct {
	Success bool           `json:"success"`
	Result  identityResult `json:"Result"`
}

type identityResult struct {
	SessionID  string              `json:"SessionId"`
	Summary    string              `json:"Summary"`
	Challenges []identityChallenge `json:"Challenges"`
}

type identityChallenge struct {
	Mechanisms []identityMechanism `json:"Mechanisms"`
}

type identityMechanism struct {
	Name        string `json:"Name"`
	MechanismID string `json:"MechanismId"`
	Image       string `json:"Image"`
}

func (c *NativeClient) identityRequest(ctx context.Context, phase, endpoint string, payload map[string]string) (identityResult, error) {
	payload["TenantId"] = c.profile.TenantID
	body, err := json.Marshal(payload)
	if err != nil {
		return identityResult{}, fmt.Errorf("%s: cannot serialize request", phase)
	}
	headers := http.Header{"X-Idap-Native-Client": {"true"}}
	data, _, err := c.request(ctx, phase, http.MethodPost, endpoint, "application/json", body, headers, false)
	if err != nil {
		return identityResult{}, err
	}
	var response identityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return identityResult{}, fmt.Errorf("%s: expected an Identity JSON response", phase)
	}
	if !response.Success {
		return identityResult{}, fmt.Errorf("%s: Identity rejected the request", phase)
	}
	return response.Result, nil
}

func (c *NativeClient) advance(ctx context.Context, session, mechanism, action string) (identityResult, error) {
	return c.identityRequest(ctx, "Identity "+action, c.profile.AdvanceAuthenticationURL, map[string]string{
		"SessionId": session, "MechanismId": mechanism, "Action": action,
	})
}

func findMechanism(result identityResult, name string) (identityMechanism, bool) {
	for _, challenge := range result.Challenges {
		for _, mechanism := range challenge.Mechanisms {
			if strings.EqualFold(mechanism.Name, name) && mechanism.MechanismID != "" {
				return mechanism, true
			}
		}
	}
	return identityMechanism{}, false
}

func checkIdentityFailure(summary string) error {
	switch strings.ToLower(summary) {
	case "failure", "loginfailure", "oobrejected", "oobfailed", "denied", "cancelled", "canceled", "rejected":
		return errors.New("Identity authentication was rejected or cancelled")
	}
	return nil
}

func (c *NativeClient) AuthenticateIdentity(ctx context.Context, username string, prompt QRPrompter) error {
	c.authenticated, c.credential = false, ""
	if strings.TrimSpace(username) == "" || prompt == nil {
		return errors.New("Identity username and QR terminal are required")
	}
	result, err := c.identityRequest(ctx, "Identity start", c.profile.StartAuthenticationURL, map[string]string{
		"User": username, "Version": "1.0",
	})
	if err != nil {
		return err
	}
	qr, ok := findMechanism(result, "QR")
	if !ok || qr.Image == "" || result.SessionID == "" {
		return errors.New("Identity did not return a usable QR challenge; check the selected profile, tenant and account enrollment (no SMS fallback)")
	}
	session, mechanism := result.SessionID, qr.MechanismID
	result, err = c.advance(ctx, session, mechanism, "StartOOB")
	if err != nil {
		return err
	}
	if err := checkIdentityFailure(result.Summary); err != nil {
		return err
	}
	if err := prompt.DisplayQR(qr.Image); err != nil {
		return fmt.Errorf("display Identity QR: %w", err)
	}
	pushStarted := false
	for {
		if result.SessionID != "" {
			session = result.SessionID
		}
		if err := checkIdentityFailure(result.Summary); err != nil {
			return err
		}
		switch result.Summary {
		case "LoginSuccess":
			c.authenticated = true
			return nil
		case "NewPackage", "StartNextChallenge":
			if otp, found := findMechanism(result, "OTP"); found && !pushStarted {
				mechanism, pushStarted = otp.MechanismID, true
				prompt.Printf("[*] Approve the push notification on your device.\n")
				result, err = c.advance(ctx, session, mechanism, "StartOOB")
				if err != nil {
					return err
				}
				continue
			}
		case "OobPending", "Pending", "":
		default:
			return errors.New("Identity returned an unexpected QR/push status")
		}
		timer := time.NewTimer(c.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("Identity QR/push approval: %w", ctx.Err())
		case <-timer.C:
		}
		result, err = c.advance(ctx, session, mechanism, "Poll")
		if err != nil {
			return err
		}
	}
}
