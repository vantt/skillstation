package auththrottle

import (
	"fmt"
	"testing"
	"time"
)

func TestThrottleBlocksWithinWindowAndBoundsKeys(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	throttle := New(func() time.Time { return now })

	for range Limit - 1 {
		throttle.Record("192.0.2.1")
	}
	if throttle.Blocked("192.0.2.1") {
		t.Fatalf("blocked after %d failures, want only from %d", Limit-1, Limit)
	}
	throttle.Record("192.0.2.1")
	if !throttle.Blocked("192.0.2.1") {
		t.Fatalf("not blocked after %d failures", Limit)
	}
	if throttle.Blocked("192.0.2.2") {
		t.Fatal("another key is blocked by the failures of the first")
	}

	now = now.Add(Window)
	if throttle.Blocked("192.0.2.1") {
		t.Fatal("still blocked once the window has passed")
	}

	// Filling the table evicts the key whose last failure is oldest, so the
	// table stays bounded and recent failures survive.
	throttle = New(func() time.Time { return now })
	for range Limit {
		throttle.Record("recent")
	}
	for index := range maxKeys {
		now = now.Add(time.Millisecond)
		throttle.Record(fmt.Sprint("key-", index))
	}
	if len(throttle.entries) > maxKeys {
		t.Fatalf("entries = %d, want at most %d", len(throttle.entries), maxKeys)
	}
	if throttle.Blocked("recent") {
		t.Fatal("the key with the oldest failure was not evicted")
	}
}
