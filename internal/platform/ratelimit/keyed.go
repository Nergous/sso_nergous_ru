package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Key identifies the subject a limit is applied to within a single policy —
// for example a peer IP, a normalized username, or a client ID.
type Key string

// String returns the key as a plain string, implementing fmt.Stringer.
func (k Key) String() string {
	return string(k)
}

// entry is one key's token bucket together with the last time it was used,
// which drives idle eviction.
type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// KeyedLimiter applies a single Policy across many keys, keeping an independent
// token bucket per key. It is safe for concurrent use. Buckets that stay idle
// longer than the policy's IdleEvict are removed by the background sweep
// started with Start.
type KeyedLimiter struct {
	policy          Policy
	cleanupInterval time.Duration
	mu              sync.Mutex
	entries         map[Key]*entry
}

// NewKeyedLimiter returns a KeyedLimiter for policy p. cleanupInterval is how
// often the idle-eviction sweep runs once Start is called. The caller is
// expected to pass already-validated, positive values (validation happens at
// the config boundary).
func NewKeyedLimiter(p Policy, cleanupInterval time.Duration) *KeyedLimiter {
	return &KeyedLimiter{
		policy:          p,
		cleanupInterval: cleanupInterval,
		entries:         make(map[Key]*entry),
	}
}

// Allow reports whether a request for key may proceed under the policy. When a
// request is denied, retryAfter is the suggested wait before the next attempt;
// on success it is zero. Allow marks the key as just seen even on denial, so an
// actively hammered key is not evicted while it is being throttled.
func (l *KeyedLimiter) Allow(key Key) (allowed bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok {
		e = &entry{
			limiter: rate.NewLimiter(rate.Limit(l.policy.RPS), l.policy.Burst),
		}
		l.entries[key] = e
	}
	e.lastSeen = time.Now()

	// Reserve, then decide immediately: a zero delay means a token was
	// available, so the request proceeds and keeps the reservation. A
	// non-zero delay means the bucket is empty; cancel the reservation so a
	// rejected request does not consume a future token. Holding l.mu across
	// Reserve and Cancel is what makes the cancel exact — no other
	// reservation can interleave. Do not move this out of the lock.
	reserve := e.limiter.Reserve()
	delay := reserve.Delay()
	if delay == 0 {
		return true, 0
	}

	reserve.Cancel()
	return false, delay
}

// Start runs the idle-eviction sweep until ctx is cancelled, then returns. It
// blocks, so it is normally launched in its own goroutine.
func (l *KeyedLimiter) Start(ctx context.Context) {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.evictIdle(time.Now())
		}
	}
}

// evictIdle removes every bucket whose last use is older than now minus the
// policy's IdleEvict.
func (l *KeyedLimiter) evictIdle(now time.Time) {
	cutoff := now.Add(-l.policy.IdleEvict)

	l.mu.Lock()
	defer l.mu.Unlock()
	for key, e := range l.entries {
		if e.lastSeen.Before(cutoff) {
			delete(l.entries, key)
		}
	}
}
