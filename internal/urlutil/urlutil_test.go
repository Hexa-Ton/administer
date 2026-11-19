package urlutil

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		in         string
		wantScheme string
		wantHost   string
		wantPath   string
		wantErr    bool
	}{
		{"example.com", "https", "example.com", "", false},
		{"  example.com  ", "https", "example.com", "", false},
		{"http://example.com", "http", "example.com", "", false},
		{"https://example.com:8443", "https", "example.com:8443", "", false},
		{"https://example.com/app/", "https", "example.com", "/app/", false},
		{"", "", "", "", true},
		{"ftp://example.com", "", "", "", true},
		{"https://", "", "", "", true},
	}
	for _, tc := range tests {
		u, err := Normalize(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Normalize(%q): expected an error, got %v", tc.in, u)
			}
			continue
		}
		if err != nil {
			t.Errorf("Normalize(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if u.Scheme != tc.wantScheme || u.Host != tc.wantHost || u.Path != tc.wantPath {
			t.Errorf("Normalize(%q) = %s://%s%s, want %s://%s%s",
				tc.in, u.Scheme, u.Host, u.Path, tc.wantScheme, tc.wantHost, tc.wantPath)
		}
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct{ base, p, want string }{
		{"", "/admin", "/admin"},
		{"/", "/admin", "/admin"},
		{"/app", "/admin", "/app/admin"},
		{"/app/", "/admin", "/app/admin"},
		{"/app", "admin", "/app/admin"},
	}
	for _, tc := range tests {
		if got := JoinPath(tc.base, tc.p); got != tc.want {
			t.Errorf("JoinPath(%q, %q) = %q, want %q", tc.base, tc.p, got, tc.want)
		}
	}
}
