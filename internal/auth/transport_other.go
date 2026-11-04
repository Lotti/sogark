//go:build !windows

package auth

import "net/http"

func newNativeTransport() http.RoundTripper {
	return http.DefaultTransport
}
