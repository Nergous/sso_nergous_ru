package auditbus

import (
	"context"
	"sync"

	"sso/internal/modules/audit"
)

// RecorderEmitter is an in-memory audit.Emitter for tests. Every
// emitted event is appended to an internal slice; Events / Len expose
// the buffer, Reset clears it. Concurrent-safe.
//
// Production code must not use RecorderEmitter — it does not persist
// anywhere and the slice grows unboundedly.
type RecorderEmitter struct {
	mu     sync.Mutex
	events []*audit.Audit
}

func NewRecorderEmitter() *RecorderEmitter {
	return &RecorderEmitter{}
}

// Emit stores the event verbatim — no sanitisation, no copy. Tests that
// need to verify the post-sanitise wire form should chain through
// Sanitize themselves.
func (r *RecorderEmitter) Emit(_ context.Context, a *audit.Audit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, a)
	return nil
}

// Events returns a defensive copy of the captured slice. The returned
// audit pointers reference the same aggregates that were emitted.
func (r *RecorderEmitter) Events() []*audit.Audit {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*audit.Audit, len(r.events))
	copy(out, r.events)
	return out
}

func (r *RecorderEmitter) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func (r *RecorderEmitter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}
