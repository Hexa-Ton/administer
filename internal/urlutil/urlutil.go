package urlutil

import (
	"fmt"
	"net/url"
	"strings"
)

// Normalize takes a raw target — a bare domain or a full URL, with
// or without scheme — and returns a validated *url.URL. A missing
// scheme defaults to https.
func Normalize(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty target")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q (use http or https)", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("no host in target %q", raw)
	}
	return u, nil
}

// JoinPath appends a wordlist path to a base URL path, keeping
// exactly one slash between them.
func JoinPath(base, p string) string {
	if base == "" {
		base = "/"
	}
	base = strings.TrimSuffix(base, "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return base + p
}
