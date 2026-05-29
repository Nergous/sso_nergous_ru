package domain

import (
	"context"
	"time"
)

// Repository is the persistence contract for recovery-code batches.
//
// Concurrency model:
//
//   - CreateBatch is expected to be called inside a transaction together
//     with the batch's code rows — the repository implementation accepts
//     the codes as part of the aggregate and writes both tables atomically.
//   - RevokeActiveBatchesForUser is the idempotent "kill all active
//     batches" used as a precondition by Generate. It is a no-op when
//     the user has nothing active.
//   - ConsumeCode performs a conditional UPDATE
//     (`SET used_at=? WHERE batch_id=? AND code_hash=? AND used_at IS NULL`).
//     A 0-rows-affected outcome surfaces ErrRecoveryCodeInvalid; this is
//     the authoritative single-use guard against concurrent consumption.
type Repository interface {
	// CreateBatch persists the batch and every Code returned by
	// Batch.Codes() in one atomic operation.
	CreateBatch(ctx context.Context, b *Batch) error

	// GetActiveBatchByUser returns the user's currently-active batch
	// (revoked_at IS NULL). Returns ErrBatchNotFound when no row matches.
	// The Batch.Codes() slice is fully populated.
	GetActiveBatchByUser(ctx context.Context, userID UserID) (*Batch, error)

	// RevokeBatch marks the batch revoked. Idempotent — already-revoked
	// rows return nil.
	RevokeBatch(ctx context.Context, batchID BatchID, now time.Time) error

	// RevokeActiveBatchesForUser revokes every non-revoked batch for the
	// user. Idempotent and bulk: zero affected rows is not an error.
	RevokeActiveBatchesForUser(ctx context.Context, userID UserID, now time.Time) error

	// ConsumeCode flips exactly one unused code from "unused" to "used".
	// Returns ErrRecoveryCodeInvalid when no matching unused row exists
	// (unknown hash, already-used, or the batch is gone).
	ConsumeCode(ctx context.Context, batchID BatchID, hash []byte, now time.Time) error
}
