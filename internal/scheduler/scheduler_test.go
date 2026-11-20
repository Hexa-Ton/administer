package scheduler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/Hexa-Ton/administer/internal/classify"
	"github.com/Hexa-Ton/administer/internal/engine"
	"github.com/Hexa-Ton/administer/internal/netx"
	"github.com/Hexa-Ton/administer/internal/scheduler"
)

// hitCounter records how many times each path was requested.
type hitCounter struct {
	mu   sync.Mutex
	hits map[string]int
}

func newHitCounter() *hitCounter {
	return &hitCounter{hits: make(map[string]int)}
}

func (h *hitCounter) add(path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hits[path]++
}

func (h *hitCounter) snapshot() map[string]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]int, len(h.hits))
	for k, v := range h.hits {
		out[k] = v
	}
	return out
}

func newEngines(t *testing.T, n int, base string, rate float64) []*engine.Engine {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	client := netx.NewClient(netx.ClientConfig{Timeout: 5 * time.Second})
	limiter := netx.NewRateLimiter(rate)
	engines := make([]*engine.Engine, n)
	for i := range engines {
		engines[i] = &engine.Engine{ID: i + 1, Client: client, Base: u, Limiter: limiter, MaxBody: 1 << 20}
	}
	return engines
}

func makeJobs(n int) []string {
	jobs := make([]string, n)
	for i := range jobs {
		jobs[i] = fmt.Sprintf("/path%03d", i)
	}
	return jobs
}

func TestEachPathIsRequestedExactlyOnce(t *testing.T) {
	counter := newHitCounter()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.add(r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	jobs := makeJobs(100)
	result := scheduler.Run(context.Background(), newEngines(t, 20, srv.URL, 0), jobs, classify.Baseline{}, nil)

	if result.Discovery != nil {
		t.Fatalf("unexpected discovery: %+v", result.Discovery)
	}
	if result.CancelReason != scheduler.NoCancel {
		t.Errorf("CancelReason = %v, want NoCancel for a scan that ran to the end", result.CancelReason)
	}
	if result.Completed != int64(len(jobs)) {
		t.Errorf("completed %d jobs, want %d", result.Completed, len(jobs))
	}
	hits := counter.snapshot()
	if len(hits) != len(jobs) {
		t.Errorf("server saw %d distinct paths, want %d", len(hits), len(jobs))
	}
	for path, n := range hits {
		if n != 1 {
			t.Errorf("path %s was requested %d times, want exactly once", path, n)
		}
	}
}

func TestFirstCandidateStopsTheScan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-r.Context().Done():
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	jobs := append([]string{"/admin"}, makeJobs(200)...)
	result := scheduler.Run(context.Background(), newEngines(t, 5, srv.URL, 0), jobs, classify.Baseline{}, nil)

	if result.Discovery == nil {
		t.Fatal("expected a discovery")
	}
	if result.Discovery.Path != "/admin" {
		t.Errorf("discovery path = %q, want /admin", result.Discovery.Path)
	}
	if result.Discovery.StatusCode != http.StatusFound {
		t.Errorf("discovery status = %d, want 302", result.Discovery.StatusCode)
	}
	if want := srv.URL + "/admin/login"; result.Discovery.Redirect != want {
		t.Errorf("discovery redirect = %q, want %q", result.Discovery.Redirect, want)
	}
	if result.CancelReason != scheduler.ReasonDiscovery {
		t.Errorf("CancelReason = %v, want ReasonDiscovery", result.CancelReason)
	}
	if result.Completed >= int64(len(jobs)) {
		t.Errorf("the scan did not stop early: %d of %d jobs completed", result.Completed, len(jobs))
	}
}

func TestInterruptIsNotReportedAsADiscovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(300 * time.Millisecond):
		case <-r.Context().Done():
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(50*time.Millisecond, cancel)

	jobs := makeJobs(200)
	result := scheduler.Run(ctx, newEngines(t, 5, srv.URL, 0), jobs, classify.Baseline{}, nil)

	if result.Discovery != nil {
		t.Errorf("an interrupted scan must not report a discovery: %+v", result.Discovery)
	}
	if result.CancelReason != scheduler.ReasonUserInterrupt {
		t.Errorf("CancelReason = %v, want ReasonUserInterrupt", result.CancelReason)
	}
	if result.Completed >= int64(len(jobs)) {
		t.Errorf("the scan did not stop: %d of %d jobs completed", result.Completed, len(jobs))
	}
}

func TestStopDoesNotWaitOutTheRateLimiter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// One request per second, and every job is /admin, so whichever engine
	// wins the single available token finds the hit straight away. The other
	// engines are then all queueing for a token that would take a full second
	// each; they must give up at once instead of sitting those waits out
	// (without that, this run would take several seconds).
	jobs := []string{"/admin", "/admin", "/admin", "/admin", "/admin", "/admin"}
	start := time.Now()
	result := scheduler.Run(context.Background(), newEngines(t, 5, srv.URL, 1), jobs, classify.Baseline{}, nil)
	elapsed := time.Since(start)

	if result.Discovery == nil {
		t.Fatal("expected a discovery")
	}
	if elapsed > 800*time.Millisecond {
		t.Errorf("stopping took %v; engines waiting on the rate limiter must exit immediately", elapsed)
	}
}
