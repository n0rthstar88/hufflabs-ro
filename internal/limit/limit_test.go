package limit

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(5, 10*time.Minute, func() time.Time { return now })
	for i := 0; i < 4; i++ {
		l.Fail("100.64.0.9")
	}
	if l.Blocked("100.64.0.9") {
		t.Fatal("blocked after 4 fails")
	}
	l.Fail("100.64.0.9")
	if !l.Blocked("100.64.0.9") {
		t.Fatal("not blocked after 5 fails")
	}
	if l.Blocked("100.64.0.10") {
		t.Fatal("other IP blocked")
	}
	now = now.Add(10*time.Minute + time.Second)
	if l.Blocked("100.64.0.9") {
		t.Fatal("still blocked after window")
	}
}

func TestLockoutLastsFullWindow(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(5, 10*time.Minute, func() time.Time { return now })
	for i := 0; i < 4; i++ {
		l.Fail("ip")
	}
	now = now.Add(9 * time.Minute)
	l.Fail("ip") // 5th failure inside the window: locked from 12:09 to 12:19
	now = now.Add(2 * time.Minute)
	if !l.Blocked("ip") {
		t.Fatal("lockout ended 2 minutes after it started")
	}
	now = now.Add(8*time.Minute + time.Second)
	if l.Blocked("ip") {
		t.Fatal("still blocked after 10 minutes")
	}
}

func TestLimiterWindowSlides(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(5, 10*time.Minute, func() time.Time { return now })
	for i := 0; i < 4; i++ {
		l.Fail("ip")
	}
	now = now.Add(11 * time.Minute) // the first four have aged out
	l.Fail("ip")
	if l.Blocked("ip") {
		t.Fatal("old failures counted")
	}
}
