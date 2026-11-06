package auth

import (
	"net/http"
	"net/url"

	"golang.org/x/net/http/httpproxy"
)

func newEnvironmentTransport() (*http.Transport, *httpproxy.Config) {
	proxyConfig := httpproxy.FromEnvironment()
	proxy := proxyConfig.ProxyFunc()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(req *http.Request) (*url.URL, error) {
		return proxy(req.URL)
	}
	return transport, proxyConfig
}
