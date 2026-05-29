package mariadb

import (
	"database/sql"
	"time"

	domain "sso/internal/modules/recoverycode/internal/domain"
	"sso/internal/modules/recoverycode/internal/mariadb/dbgen"
)

// batchAndCodes restores a Batch aggregate from the parent row plus
// every child code row. Used by GetActiveBatchByUser.
func batchAndCodes(row dbgen.RecoveryCodeBatch, codeRows []dbgen.RecoveryCode) *domain.Batch {
	codes := make([]domain.Code, 0, len(codeRows))
	for _, r := range codeRows {
		var usedAt time.Time
		if r.UsedAt.Valid {
			usedAt = r.UsedAt.Time
		}
		codes = append(codes, domain.RestoreCode(r.CodeHash, usedAt))
	}
	var revokedAt time.Time
	if row.RevokedAt.Valid {
		revokedAt = row.RevokedAt.Time
	}
	return domain.RestoreBatch(domain.RestoreBatchParams{
		ID:          domain.BatchID(row.ID),
		UserID:      domain.UserID(row.UserID),
		GeneratedAt: row.GeneratedAt,
		RevokedAt:   revokedAt,
		Codes:       codes,
	})
}

func toCreateBatchParams(b *domain.Batch) dbgen.CreateRecoveryCodeBatchParams {
	return dbgen.CreateRecoveryCodeBatchParams{
		ID:          b.ID().String(),
		UserID:      b.UserID().String(),
		GeneratedAt: b.GeneratedAt(),
		RevokedAt:   revokedAtToDB(b.RevokedAt()),
	}
}

func toCreateCodeParams(batchID domain.BatchID, c domain.Code) dbgen.CreateRecoveryCodeParams {
	return dbgen.CreateRecoveryCodeParams{
		BatchID:  batchID.String(),
		CodeHash: c.Hash(),
		UsedAt:   revokedAtToDB(c.UsedAt()),
	}
}

func revokedAtToDB(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}
