//go:build !windows

package auth

import "testing"

func TestExtractSAMLFromPostData(t *testing.T) {
	tests := []struct {
		name     string
		postData string
		want     string
	}{
		{
			name:     "url encoded form",
			postData: "RelayState=%2Fapp&SAMLResponse=PHNhbWxwOlJlc3BvbnNlPg%3D%3D",
			want:     "PHNhbWxwOlJlc3BvbnNlPg==",
		},
		{
			name:     "single field",
			postData: "SAMLResponse=raw-token",
			want:     "raw-token",
		},
		{
			name:     "missing field",
			postData: "RelayState=/app",
			want:     "",
		},
		{
			name:     "empty body",
			postData: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSAMLFromPostData(tt.postData)
			if got != tt.want {
				t.Fatalf("extractSAMLFromPostData(%q) = %q, want %q", tt.postData, got, tt.want)
			}
		})
	}
}
