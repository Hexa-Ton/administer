package netx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRedirectsNotFollowedByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/landing", http.StatusFound)
			return
		}
		fmt.Fprint(w, "landing")
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Timeout: 5 * time.Second})
	resp, err := client.Get(srv.URL + "/start")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302: the redirect must be reported, not followed", resp.StatusCode)
	}
}

func TestFollowRedirectsWithinTheSameHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/landing", http.StatusFound)
			return
		}
		fmt.Fprint(w, "landing")
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Timeout: 5 * time.Second, FollowRedirects: true})
	resp, err := client.Get(srv.URL + "/start")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 after following the redirect", resp.StatusCode)
	}
}

func TestFollowRedirectsStopsAtTenHops(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Timeout: 5 * time.Second, FollowRedirects: true})
	resp, err := client.Get(srv.URL + "/loop")
	if err != nil {
		t.Fatalf("a redirect loop must end with the last response, not an error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want the last 302", resp.StatusCode)
	}
	if n := hits.Load(); n > 11 {
		t.Errorf("server was hit %d times, redirect following must stop after 10 hops", n)
	}
}

func TestRateLimiterThrottles(t *testing.T) {
	limiter := NewRateLimiter(100)
	start := time.Now()
	for i := 0; i < 200; i++ {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	// The first 100 are an instant burst; the other 100 must be paced at
	// 100/second, so the whole run cannot finish much under a second.
	if elapsed := time.Since(start); elapsed < 700*time.Millisecond {
		t.Errorf("200 requests at 100/s finished in %v: the limiter is not throttling", elapsed)
	}
}

func TestRateLimiterDisabledNeverBlocks(t *testing.T) {
	limiter := NewRateLimiter(0)
	start := time.Now()
	for i := 0; i < 10000; i++ {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("a disabled limiter took %v for 10000 calls", elapsed)
	}
}

func TestRateLimiterWaitHonoursCancellation(t *testing.T) {
	limiter := NewRateLimiter(1)
	if err := limiter.Wait(context.Background()); err != nil { // uses the one available token
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := limiter.Wait(ctx); err == nil {
		t.Fatal("expected the wait to be cancelled")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("cancellation took %v; a cancelled wait must return promptly", elapsed)
	}
}
