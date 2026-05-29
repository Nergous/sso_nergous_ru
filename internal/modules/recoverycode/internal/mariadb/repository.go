package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	domain "sso/internal/modules/recoverycode/internal/domain"
	"sso/internal/modules/recoverycode/internal/mariadb/dbgen"
)

type Repository struct {
	db *sql.DB
	q  *dbgen.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: dbgen.New(db)}
}

var _ domain.Repository = (*Repository)(nil)

// CreateBatch persists the batch and every Code from b.Codes() in a
// single transaction. A partial write would leave a batch row with
// fewer codes than the user was shown — single-use semantics would then
// look like "code invalid" for codes the user actually got.
func (r *Repository) CreateBatch(ctx context.Context, b *domain.Batch) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recovery code repo: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	q := r.q.WithTx(tx)

	if err := q.CreateRecoveryCodeBatch(ctx, toCreateBatchParams(b)); err != nil {
		return fmt.Errorf("recovery code repo: create batch: %w", err)
	}
	for _, c := range b.Codes() {
		if err := q.CreateRecoveryCode(ctx, toCreateCodeParams(b.ID(), c)); err != nil {
			return fmt.Errorf("recovery code repo: create code: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("recovery code repo: commit: %w", err)
	}
	return nil
}

func (r *Repository) GetActiveBatchByUser(ctx context.Context, userID domain.UserID) (*domain.Batch, error) {
	row, err := r.q.GetActiveRecoveryBatchByUser(ctx, userID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrBatchNotFound
		}
		return nil, fmt.Errorf("recovery code repo: get_active_batch: %w", err)
	}
	codes, err := r.q.ListRecoveryCodesByBatch(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("recovery code repo: list_codes: %w", err)
	}
	return batchAndCodes(row, codes), nil
}

func (r *Repository) RevokeBatch(ctx context.Context, batchID domain.BatchID, now time.Time) error {
	_, err := r.q.RevokeRecoveryBatch(ctx, dbgen.RevokeRecoveryBatchParams{
		RevokedAt: revokedAtToDB(now),
		ID:        batchID.String(),
	})
	if err != nil {
		return fmt.Errorf("recovery code repo: revoke_batch: %w", err)
	}
	return nil
}

func (r *Repository) RevokeActiveBatchesForUser(ctx context.Context, userID domain.UserID, now time.Time) error {
	_, err := r.q.RevokeActiveRecoveryBatchesForUser(ctx, dbgen.RevokeActiveRecoveryBatchesForUserParams{
		RevokedAt: revokedAtToDB(now),
		UserID:    userID.String(),
	})
	if err != nil {
		return fmt.Errorf("recovery code repo: revoke_active_for_user: %w", err)
	}
	return nil
}

// ConsumeCode atomically flips one (batch, hash) row from unused to used.
// The UPDATE's WHERE clause includes `used_at IS NULL`, so a concurrent
// Consume of the same hash sees zero affected rows on the second attempt
// — the single-use invariant is enforced at the row level, not by a
// read-then-write in Go.
func (r *Repository) ConsumeCode(ctx context.Context, batchID domain.BatchID, hash []byte, now time.Time) error {
	res, err := r.q.ConsumeRecoveryCode(ctx, dbgen.ConsumeRecoveryCodeParams{
		UsedAt:   revokedAtToDB(now),
		BatchID:  batchID.String(),
		CodeHash: hash,
	})
	if err != nil {
		return fmt.Errorf("recovery code repo: consume: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("recovery code repo: consume: rows_affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrRecoveryCodeInvalid
	}
	return nil
}
