package netx

import (
	"context"
	"sync"
	"time"
)

// RateLimiter is a minimal stdlib-only token bucket shared across
// engine instances. A non-positive rate disables limiting entirely
// (requests are then bounded only by engine concurrency).
type RateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	rate     float64
	last     time.Time
	disabled bool
}

// NewRateLimiter returns a limiter allowing requestsPerSecond requests per
// second (with a burst of the same size). Zero or negative disables it.
func NewRateLimiter(requestsPerSecond float64) *RateLimiter {
	if requestsPerSecond <= 0 {
		return &RateLimiter{disabled: true}
	}
	return &RateLimiter{
		tokens: requestsPerSecond,
		max:    requestsPerSecond,
		rate:   requestsPerSecond,
		last:   time.Now(),
	}
}

// Wait blocks until a request may be sent, or until ctx is done, in which
// case it returns ctx's error. Honouring ctx matters: when the scan stops
// (first hit or Ctrl+C) engines waiting for a token must exit at once
// instead of sitting out the rest of their wait.
func (r *RateLimiter) Wait(ctx context.Context) error {
	if r == nil || r.disabled {
		return nil
	}
	for {
		r.mu.Lock()
		now := time.Now()
		r.tokens += now.Sub(r.last).Seconds() * r.rate
		r.last = now
		if r.tokens > r.max {
			r.tokens = r.max
		}
		if r.tokens >= 1 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}
		wait := time.Duration((1 - r.tokens) / r.rate * float64(time.Second))
		r.mu.Unlock()

		if wait < time.Millisecond {
			wait = time.Millisecond
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
