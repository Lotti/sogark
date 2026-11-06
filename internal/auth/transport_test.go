package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func clearProxyEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy", "REQUEST_METHOD"} {
		t.Setenv(name, "")
	}
}

func TestAuthenticationEnvironmentProxySelection(t *testing.T) {
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		t.Run(name, func(t *testing.T) {
			clearProxyEnvironment(t)
			t.Setenv(name, "http://proxy.example.test:3128")
			transport, _ := newEnvironmentTransport()
			scheme := "https"
			if name == "HTTP_PROXY" || name == "http_proxy" {
				scheme = "http"
			}
			request, _ := http.NewRequest(http.MethodGet, scheme+"://identity.example.test/start", nil)
			proxy, err := transport.Proxy(request)
			if err != nil || proxy == nil || proxy.String() != "http://proxy.example.test:3128" {
				t.Fatalf("proxy=%v error=%v", proxy, err)
			}
		})
	}
}

func TestAuthenticationProxyBypassAndClientSnapshot(t *testing.T) {
	clearProxyEnvironment(t)
	t.Setenv("HTTPS_PROXY", "http://first.example.test:3128")
	t.Setenv("no_proxy", "identity.example.test")
	first, _ := newEnvironmentTransport()
	request, _ := http.NewRequest(http.MethodGet, "https://identity.example.test/start", nil)
	if proxy, err := first.Proxy(request); err != nil || proxy != nil {
		t.Fatalf("NO_PROXY bypass lost: proxy=%v error=%v", proxy, err)
	}
	request, _ = http.NewRequest(http.MethodGet, "https://vault.example.test/cache", nil)
	t.Setenv("HTTPS_PROXY", "http://second.example.test:3128")
	second, _ := newEnvironmentTransport()
	for _, test := range []struct {
		transport *http.Transport
		want      string
	}{{first, "first.example.test:3128"}, {second, "second.example.test:3128"}} {
		proxy, err := test.transport.Proxy(request)
		if err != nil || proxy == nil || proxy.Host != test.want {
			t.Fatalf("proxy settings must be captured per client: proxy=%v error=%v", proxy, err)
		}
	}
}

func TestITAuthenticationRequestsUseEnvironmentProxy(t *testing.T) {
	clearProxyEnvironment(t)
	var requests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Host != "identity.example.test" && r.URL.Host != "vault.example.test" {
			t.Errorf("unexpected authentication destination %q", r.URL.Host)
		}
		if !r.URL.IsAbs() {
			t.Error("request did not reach the HTTP proxy in absolute form")
		}
		fmt.Fprint(w, `{}`)
	}))
	defer proxy.Close()
	t.Setenv("http_proxy", proxy.URL)
	client, err := NewNativeClient(nativeProfile("http://localhost", "oidc"))
	if err != nil {
		t.Fatal(err)
	}
	// Synthetic origins avoid localhost's automatic proxy bypass.
	client.identityURL, _ = url.Parse("http://identity.example.test")
	client.pvwaURL, _ = url.Parse("http://vault.example.test")
	for _, endpoint := range []string{
		"http://identity.example.test/start", "http://identity.example.test/advance",
		"http://vault.example.test/authorize", "http://vault.example.test/token",
		"http://vault.example.test/cache",
	} {
		if _, _, err := client.request(context.Background(), "IT authentication fixture", http.MethodPost, endpoint, "application/json", []byte(`{}`), nil, false); err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 5 {
		t.Fatalf("expected all five authentication/key requests through proxy, got %d", requests.Load())
	}
}
