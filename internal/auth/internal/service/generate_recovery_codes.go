package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sso/internal/audit"
	"sso/internal/identity"
	"sso/internal/recoverycode"
	"sso/internal/kernel/actor"
)

// GenerateRecoveryCodesInput carries the caller's own subject id; the
// proto contract scopes generation to the caller's own account.
type GenerateRecoveryCodesInput struct {
	UserID string
}

// GenerateRecoveryCodesOutput is the use-case view of one fresh batch.
// Codes are plaintext, shown to the user once; the server retains only
// SHA-256 digests on the recovery_codes table.
type GenerateRecoveryCodesOutput struct {
	Codes       []string
	GeneratedAt time.Time
}

// GenerateRecoveryCodes issues a fresh batch of ten one-time codes for
// the caller, revoking any previously-active batch first (proto
// contract: "ANY previously issued codes are invalidated atomically").
//
// State checks mirror ChangePassword: BLOCKED and DELETED users are
// surfaced (proto: FAILED_PRECONDITION). A vanished user surfaces as
// INVALID_TOKEN — the access_token outlived the account.
func (s *Service) GenerateRecoveryCodes(
	ctx context.Context, in GenerateRecoveryCodesInput,
) (GenerateRecoveryCodesOutput, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return GenerateRecoveryCodesOutput{}, err
	}
	userID, err := identity.ParseUserID(in.UserID)
	if err != nil {
		return GenerateRecoveryCodesOutput{}, err
	}

	aud := audit.BaseFromActor(a, audit.EventTypeAuthGenerateRecoveryCodes)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = userID.String()

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonInvalidToken)
			return GenerateRecoveryCodesOutput{}, ErrInvalidToken
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return GenerateRecoveryCodesOutput{}, fmt.Errorf("generate recovery codes: get user: %w", err)
	}

	switch user.Status() {
	case identity.UserStatusDeleted:
		s.auditor.Deny(ctx, aud, audit.ReasonUserDeleted)
		return GenerateRecoveryCodesOutput{}, ErrUserDeleted
	case identity.UserStatusBlocked:
		s.auditor.Deny(ctx, aud, audit.ReasonUserBlocked)
		return GenerateRecoveryCodesOutput{}, ErrUserBlocked
	}

	now := s.now().UTC()

	// Revoke first — keeps the "max one active batch per user" invariant
	// honest even if a previous Generate raced with itself. Bulk and
	// idempotent: no-op when the user has no active batch.
	if err := s.recoveryCodes.RevokeActiveBatchesForUser(ctx, recoverycode.UserID(user.ID().String()), now); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return GenerateRecoveryCodesOutput{}, fmt.Errorf("generate recovery codes: revoke active batches: %w", err)
	}

	codes, hashes, err := s.recoveryGen.Generate()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return GenerateRecoveryCodesOutput{}, fmt.Errorf("generate recovery codes: gen: %w", err)
	}

	batchID, err := recoverycode.NewBatchID()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return GenerateRecoveryCodesOutput{}, fmt.Errorf("generate recovery codes: new batch id: %w", err)
	}

	batch := recoverycode.NewBatch(recoverycode.NewBatchParams{
		ID:          batchID,
		UserID:      recoverycode.UserID(user.ID().String()),
		GeneratedAt: now,
		CodeHashes:  hashes,
	})

	if err := s.recoveryCodes.CreateBatch(ctx, batch); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return GenerateRecoveryCodesOutput{}, fmt.Errorf("generate recovery codes: persist batch: %w", err)
	}

	s.auditor.Success(ctx, aud)

	return GenerateRecoveryCodesOutput{
		Codes:       codes,
		GeneratedAt: now,
	}, nil
}
