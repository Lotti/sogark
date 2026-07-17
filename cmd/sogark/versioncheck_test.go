package main

import "testing"

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		candidate, current string
		want               bool
	}{
		{"v1.2.3", "v1.2.2", true},
		{"v1.3.0", "v1.2.9", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.2.3", "v1.2.3", false}, // same version
		{"v1.2.2", "v1.2.3", false}, // older
		{"v1.0.0", "v2.0.0", false}, // major older
		{"v1.2.3-beta", "v1.2.2", true},  // pre-release suffix stripped
		{"v1.2.3", "dev", false},          // unparseable current → false
		{"", "v1.0.0", false},             // empty candidate → false
		{"not-a-version", "v1.0.0", false},
	}

	for _, tt := range tests {
		got := isNewerVersion(tt.candidate, tt.current)
		if got != tt.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v",
				tt.candidate, tt.current, got, tt.want)
		}
	}
}
