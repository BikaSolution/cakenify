package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a simple fixed-window limiter keyed by an arbitrary
// string (client IP, username, user ID, ...). Good enough for a
// single-instance hobby app — no need for a shared/distributed limiter.
type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{attempts: make(map[string][]time.Time), limit: limit, window: window}
}

// Allow reports whether key is still under the limit, recording this
// attempt if so.
func (rl *rateLimiter) Allow(key string) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := now.Add(-rl.window)
	kept := rl.attempts[key][:0]
	for _, t := range rl.attempts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= rl.limit {
		rl.attempts[key] = kept
		return false
	}

	rl.attempts[key] = append(kept, now)

	return true
}

// clientIP resolves the real client address. cakenify is only ever meant
// to be reached through the cloudflared tunnel, which sets CF-Connecting-IP
// on every request it forwards — that header is only trustworthy because
// docker-compose.yml binds the app's host port to loopback, so it can't be
// reached (and CF-Connecting-IP can't be spoofed) directly from the network.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
