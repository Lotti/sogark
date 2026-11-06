//go:build !windows

package auth

import "net/http"

func newNativeTransport() http.RoundTripper {
	transport, _ := newEnvironmentTransport()
	return transport
}
