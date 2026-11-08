//go:build windows

package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type testRoundTripper func(*http.Request) (*http.Response, error)

type trackedRequestBody struct {
	io.Reader
	closed bool
}

func TestWindowsProxyIntegratedFallback(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) {
			clearProxyEnvironment(t)
			t.Setenv("HTTP_PROXY", "http://fixture:password@proxy.example.test:8080")
			t.Setenv("HTTPS_PROXY", "http://fixture:password@proxy.example.test:8080")
			_, settings := newEnvironmentTransport()
			var nativeCalls int
			transport := &windowsProxyTransport{
				proxyConfig: settings,
				env: testRoundTripper(func(req *http.Request) (*http.Response, error) {
					if req.Body != nil {
						req.Body.Close()
					}
					if scheme == "https" {
						return nil, errProxyAuthenticationRequired
					}
					return &http.Response{StatusCode: 407, Body: io.NopCloser(strings.NewReader(""))}, nil
				}),
				namedProxy: func(proxy *url.URL) http.RoundTripper {
					if proxy.User != nil || proxy.Host != "proxy.example.test:8080" {
						t.Fatalf("native retry must use the same proxy without supplied credentials")
					}
					return testRoundTripper(func(req *http.Request) (*http.Response, error) {
						nativeCalls++
						body, _ := io.ReadAll(req.Body)
						req.Body.Close()
						if string(body) != `{"fixture":true}` {
							t.Errorf("retry body lost: %q", body)
						}
						return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
					})
				},
			}
			req, _ := http.NewRequest(http.MethodPost, scheme+"://identity.example.test/start", strings.NewReader(`{"fixture":true}`))
			resp, err := transport.RoundTrip(req)
			if err != nil || resp.StatusCode != 200 || nativeCalls != 1 {
				t.Fatalf("integrated fallback failed: response=%v error=%v calls=%d", resp, err, nativeCalls)
			}
		})
	}
}

func TestWindowsProxyBypassDoesNotUseSystemProxy(t *testing.T) {
	clearProxyEnvironment(t)
	t.Setenv("HTTPS_PROXY", "http://proxy.example.test:8080")
	t.Setenv("NO_PROXY", "identity.example.test")
	_, settings := newEnvironmentTransport()
	direct := errors.New("direct route")
	transport := &windowsProxyTransport{
		proxyConfig: settings,
		env:         testRoundTripper(func(*http.Request) (*http.Response, error) { return nil, direct }),
		system:      testRoundTripper(func(*http.Request) (*http.Response, error) { t.Fatal("NO_PROXY ignored"); return nil, nil }),
	}
	req, _ := http.NewRequest(http.MethodGet, "https://identity.example.test/start", nil)
	if _, err := transport.RoundTrip(req); !errors.Is(err, direct) {
		t.Fatalf("bypass failed: %v", err)
	}
}

func (b *trackedRequestBody) Close() error {
	b.closed = true
	return nil
}

func (f testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWindowsProxyTransportRoundTrip(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/identity/start?fixture=yes" {
			t.Errorf("unexpected request target %s %s", r.Method, r.URL.RequestURI())
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request headers were not forwarded: %v", r.Header)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != `{"fixture":true}` {
			t.Errorf("request body was not forwarded: %q, %v", body, err)
		}
		w.Header().Add("Set-Cookie", "one=1")
		w.Header().Add("Set-Cookie", "two=2")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		server.URL+"/identity/start?fixture=yes",
		strings.NewReader(`{"fixture":true}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	requestBody := &trackedRequestBody{Reader: req.Body}
	req.Body = requestBody
	resp, err := newNativeTransport().RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if !requestBody.closed {
		t.Fatal("WinHTTP transport did not close the request body")
	}
	if resp.StatusCode != http.StatusCreated || len(resp.Header.Values("Set-Cookie")) != 2 {
		t.Fatalf("unexpected response metadata: status=%d headers=%v", resp.StatusCode, resp.Header)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "ok" {
		t.Fatalf("unexpected response body %q: %v", body, err)
	}
}

func TestWindowsProxyTransportHonorsEnvironmentProxyForScheme(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:3128")
	t.Setenv("HTTPS_PROXY", "")
	transport := newNativeTransport().(*windowsProxyTransport)
	if !transport.hasEnvironmentProxy(&url.URL{Scheme: "http"}) {
		t.Fatal("HTTP_PROXY should override the Windows system proxy for HTTP")
	}
	if transport.hasEnvironmentProxy(&url.URL{Scheme: "https"}) {
		t.Fatal("HTTP_PROXY must not override the Windows system proxy for HTTPS")
	}
}

func TestWindowsProxyTransportDispatchesPerScheme(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:3128")
	t.Setenv("HTTPS_PROXY", "")
	envErr := errors.New("environment transport")
	systemErr := errors.New("system transport")
	_, proxyConfig := newEnvironmentTransport()
	transport := &windowsProxyTransport{
		proxyConfig: proxyConfig,
		namedProxy: func(*url.URL) http.RoundTripper {
			return testRoundTripper(func(*http.Request) (*http.Response, error) { return nil, envErr })
		},
		env: testRoundTripper(func(*http.Request) (*http.Response, error) {
			return nil, envErr
		}),
		system: testRoundTripper(func(*http.Request) (*http.Response, error) {
			return nil, systemErr
		}),
	}

	for _, test := range []struct {
		scheme string
		want   error
	}{
		{scheme: "http", want: envErr},
		{scheme: "https", want: systemErr},
	} {
		req, err := http.NewRequest(http.MethodGet, test.scheme+"://example.test", nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := transport.RoundTrip(req); !errors.Is(err, test.want) {
			t.Errorf("%s request selected wrong transport: %v", test.scheme, err)
		}
	}
}

func TestWindowsProxyTransportLeavesRedirectHandlingToHTTPClient(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	var redirected bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected = true
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, source.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := newNativeTransport().RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != target.URL || redirected {
		t.Fatal("WinHTTP followed a redirect outside net/http redirect policy")
	}
}
