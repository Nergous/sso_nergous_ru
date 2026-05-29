package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Key string

func NewKey(s string) *Key {
	k := Key(s)
	return &k
}

func (k *Key) String() string {
	return string(*k)
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type KeyedLimiter struct {
	policy          Policy
	cleanupInterval time.Duration
	mu              sync.Mutex
	entries         map[Key]*entry
}

func NewKeyedLimiter(p Policy, cleanupInterval time.Duration) *KeyedLimiter {
	return &KeyedLimiter{
		policy:          p,
		cleanupInterval: cleanupInterval,
		mu:              sync.Mutex{},
		entries:         make(map[Key]*entry),
	}
}

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

	reserve := e.limiter.Reserve()
	delay := reserve.Delay()
	if delay == 0 {
		return true, 0
	}

	reserve.Cancel()
	return false, delay
}

func (l *KeyedLimiter) Start(ctx context.Context) error {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			l.evictIdle(time.Now())
		}
	}
}

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
