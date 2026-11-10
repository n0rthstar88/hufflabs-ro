// Package limit locks out a source IP after too many wrong invite codes.
package limit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	fails  map[string][]time.Time
	locked map[string]time.Time // lockout end per IP
}

func NewLimiter(max int, window time.Duration, now func() time.Time) *Limiter {
	return &Limiter{max: max, window: window, now: now, fails: map[string][]time.Time{}, locked: map[string]time.Time{}}
}

// recent drops failures older than the window. Caller holds mu.
func (l *Limiter) recent(ip string) []time.Time {
	cut := l.now().Add(-l.window)
	kept := l.fails[ip][:0]
	for _, t := range l.fails[ip] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, ip)
		return nil
	}
	l.fails[ip] = kept
	return kept
}

// Blocked reports whether ip is inside a lockout. A lockout starts when ip reaches
// max failures inside the window and lasts one full window from then.
func (l *Limiter) Blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	until, ok := l.locked[ip]
	if ok && l.now().Before(until) {
		return true
	}
	delete(l.locked, ip)
	return false
}

func (l *Limiter) Fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := append(l.recent(ip), l.now())
	l.fails[ip] = f
	if len(f) >= l.max {
		l.locked[ip] = l.now().Add(l.window)
		delete(l.fails, ip)
	}
}
