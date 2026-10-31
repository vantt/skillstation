// Package auththrottle slows down repeated failed authentications. The web UI
// server and the remote MCP gate share it so both refuse a client the same
// way.
package auththrottle

import (
	"net"
	"sync"
	"time"
)

const (
	// Limit is the number of failures within Window after which a key is
	// refused until its oldest failure leaves the window.
	Limit = 20
	// Window is how long a failure counts.
	Window = time.Minute
	// maxKeys bounds memory: a new key beyond it evicts the key whose last
	// failure is oldest.
	maxKeys = 1024
)

// Throttle counts failed authentications per key. The zero value is not
// usable; call New.
type Throttle struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string][]time.Time
}

// New returns an empty throttle that reads the time from now (time.Now when
// nil).
func New(now func() time.Time) *Throttle {
	if now == nil {
		now = time.Now
	}
	return &Throttle{now: now, entries: make(map[string][]time.Time)}
}

// RemoteHost returns the host part of a request's remote address, or the
// address itself when it has no port.
func RemoteHost(remoteAddress string) string {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		return remoteAddress
	}
	return host
}

// Blocked reports whether key has Limit or more failures within Window.
func (throttle *Throttle) Blocked(key string) bool {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	return len(throttle.recent(key)) >= Limit
}

// Record counts one failure for key.
func (throttle *Throttle) Record(key string) {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	if _, known := throttle.entries[key]; !known && len(throttle.entries) >= maxKeys {
		throttle.evictOldest()
	}
	throttle.entries[key] = append(throttle.recent(key), throttle.now())
}

// recent drops expired failures for key and returns the rest. The caller
// holds the lock.
func (throttle *Throttle) recent(key string) []time.Time {
	cutoff := throttle.now().Add(-Window)
	kept := throttle.entries[key][:0]
	for _, at := range throttle.entries[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(throttle.entries, key)
		return nil
	}
	throttle.entries[key] = kept
	return kept
}

func (throttle *Throttle) evictOldest() {
	oldestKey := ""
	var oldest time.Time
	for key, times := range throttle.entries {
		last := times[len(times)-1]
		if oldestKey == "" || last.Before(oldest) {
			oldestKey, oldest = key, last
		}
	}
	delete(throttle.entries, oldestKey)
}
