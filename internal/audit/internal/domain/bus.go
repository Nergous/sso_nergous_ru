package domain

import "context"

// Emitter is the publication surface used by use-cases to record an audit
// event. The implementation chooses the strategy: synchronous write,
// async fan-out, batching, sampling, etc. Use-cases stay oblivious.
//
// Contract:
//   - Emit must not block business-critical paths in a way that affects
//     correctness; failures are typically logged, not surfaced.
//   - Implementations must not mutate the passed Audit.
type Emitter interface {
	Emit(ctx context.Context, a *Audit) error
}

// NopEmitter is a no-op Emitter for tests / bootstraps where the audit
// pipeline is not yet wired.
type NopEmitter struct{}

func (NopEmitter) Emit(context.Context, *Audit) error { return nil }
