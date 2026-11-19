package netx

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

type ClientConfig struct {
	Timeout         time.Duration
	Insecure        bool
	FollowRedirects bool
}

// NewClient builds an HTTP client tuned for path-discovery scanning:
// connection reuse on, TLS verification on unless explicitly
// disabled, and redirects not followed unless explicitly requested
// (and even then, never leaving the original host).
func NewClient(cfg ClientConfig) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.Timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     30 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	if cfg.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			if len(via) > 0 && req.URL.Hostname() != via[0].URL.Hostname() {
				return http.ErrUseLastResponse
			}
			return nil
		}
	} else {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return client
}
