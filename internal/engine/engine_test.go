package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/YOUR_GITHUB_USERNAME/administer/internal/classify"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/netx"
)

func newTestEngine(t *testing.T, base string) *Engine {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	return &Engine{
		ID:      1,
		Client:  netx.NewClient(netx.ClientConfig{Timeout: 5 * time.Second}),
		Base:    u,
		Limiter: netx.NewRateLimiter(0),
		MaxBody: 1 << 20,
	}
}

func TestCheckClassifiesResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/found" {
			w.Write([]byte("hello"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	e := newTestEngine(t, srv.URL)

	res := e.Check(context.Background(), "/found", classify.Baseline{})
	if res.Verdict != classify.Candidate || res.StatusCode != http.StatusOK {
		t.Errorf("/found: verdict %v, status %d; want candidate, 200", res.Verdict, res.StatusCode)
	}
	res = e.Check(context.Background(), "/missing", classify.Baseline{})
	if res.Verdict != classify.NotFound || res.StatusCode != http.StatusNotFound {
		t.Errorf("/missing: verdict %v, status %d; want not-found, 404", res.Verdict, res.StatusCode)
	}
}

func TestCheckResolvesRelativeRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/landing")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	e := newTestEngine(t, srv.URL)
	res := e.Check(context.Background(), "/start", classify.Baseline{})

	if want := srv.URL + "/landing"; res.Redirect != want {
		t.Errorf("Redirect = %q, want the full URL %q", res.Redirect, want)
	}
}

func TestCheckAppendsPathBeneathTheBaseURL(t *testing.T) {
	var mu sync.Mutex
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = r.URL.Path
		mu.Unlock()
		http.NotFound(w, r)
	}))
	defer srv.Close()

	e := newTestEngine(t, srv.URL+"/app/")
	e.Check(context.Background(), "/admin", classify.Baseline{})

	mu.Lock()
	defer mu.Unlock()
	if seen != "/app/admin" {
		t.Errorf("server saw %q, want /app/admin", seen)
	}
}

func TestCheckWithCancelledContextReturnsAnError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := newTestEngine(t, srv.URL).Check(ctx, "/admin", classify.Baseline{})
	if res.Err == nil {
		t.Error("expected an error for a cancelled context")
	}
	if res.Verdict != classify.NotFound {
		t.Errorf("a failed request must never count as a candidate, got %v", res.Verdict)
	}
}
