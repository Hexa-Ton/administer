package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Hexa-Ton/administer/internal/classify"
	"github.com/Hexa-Ton/administer/internal/netx"
	"github.com/Hexa-Ton/administer/internal/urlutil"
	"github.com/Hexa-Ton/administer/internal/version"
)

// Engine is one concurrent execution unit: it fetches a single path,
// classifies the response, and returns a Result. Up to -engines
// instances run concurrently against a shared job queue; the
// scheduler guarantees no two instances are ever given the same
// path.
type Engine struct {
	ID      int
	Client  *http.Client
	Base    *url.URL
	Limiter *netx.RateLimiter
	MaxBody int64
}

// Result is the outcome of checking one path.
type Result struct {
	EngineID   int
	Path       string
	URL        string
	StatusCode int
	Verdict    classify.Verdict
	Redirect   string
	Err        error
	Duration   time.Duration
}

// Check requests one path beneath the engine's base URL and classifies the
// response against the wildcard baseline.
func (e *Engine) Check(ctx context.Context, path string, baseline classify.Baseline) Result {
	start := time.Now()

	target := *e.Base
	target.Path = urlutil.JoinPath(e.Base.Path, path)
	reqURL := target.String()

	if err := e.Limiter.Wait(ctx); err != nil {
		return Result{EngineID: e.ID, Path: path, URL: reqURL, Err: err, Duration: time.Since(start)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return Result{EngineID: e.ID, Path: path, URL: reqURL, Err: err, Duration: time.Since(start)}
	}
	req.Header.Set("User-Agent", "administer/"+version.Version)

	resp, err := e.Client.Do(req)
	if err != nil {
		return Result{EngineID: e.ID, Path: path, URL: reqURL, Err: err, Duration: time.Since(start)}
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, e.MaxBody)
	body, _ := io.ReadAll(limited)
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])

	verdict := classify.Classify(resp.StatusCode, hash, baseline)

	// Location is often relative ("/login"); resolve it against the
	// request URL so the result is always a full, usable URL.
	redirect := ""
	if loc, locErr := resp.Location(); locErr == nil {
		redirect = loc.String()
	}

	return Result{
		EngineID:   e.ID,
		Path:       path,
		URL:        reqURL,
		StatusCode: resp.StatusCode,
		Verdict:    verdict,
		Redirect:   redirect,
		Duration:   time.Since(start),
	}
}
