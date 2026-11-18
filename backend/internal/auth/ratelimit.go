package auth

import (
	"sync"
	"time"
)

// Limits for failed logins and password reset requests. Both are counted per
// username or email address, so they work without knowing the client's IP.
const (
	maxFailedLogins  = 10
	loginWindow      = 15 * time.Minute
	maxResetRequests = 3
	resetWindow      = time.Hour
)

// limiter counts events per key inside a sliding time window. It is kept in
// memory, so counts start again after a server restart.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	events map[string][]time.Time
	swept  time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, events: make(map[string][]time.Time)}
}

// blocked reports whether key has reached the limit within the window.
func (l *limiter) blocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	return len(l.recent(key, now)) >= l.max
}

// record counts one event for key.
func (l *limiter) record(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	l.events[key] = append(l.recent(key, now), now)
}

// reset forgets all events of key, for example after a successful login.
func (l *limiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.events, key)
}

// recent returns the events of key inside the window and drops the older ones.
func (l *limiter) recent(key string, now time.Time) []time.Time {
	var kept []time.Time
	for _, at := range l.events[key] {
		if now.Sub(at) < l.window {
			kept = append(kept, at)
		}
	}
	l.events[key] = kept
	return kept
}

// sweep removes keys without recent events. It runs at most once per window,
// so that the map does not grow with every name ever tried.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.swept) < l.window {
		return
	}
	l.swept = now
	for key := range l.events {
		if len(l.recent(key, now)) == 0 {
			delete(l.events, key)
		}
	}
}
