package domain

import "errors"

var (
	// ErrBatchNotFound — no active (non-revoked) batch exists for the
	// supplied user. The use-case folds this into ErrRecoveryCodeInvalid
	// when surfacing to the auth layer: "no batch" and "wrong code" must
	// look identical on the wire so the response gives no enumeration
	// signal about whether the user ever asked for codes.
	ErrBatchNotFound = errors.New("recoverycode: batch not found")

	// ErrRecoveryCodeInvalid — the supplied (batch, hash) pair did not
	// match an unused row. Covers all three failure modes: unknown hash,
	// already-used hash, revoked batch. Maps to UNAUTHENTICATED +
	// ERROR_REASON_RECOVERY_CODE_INVALID on the wire (per errors.proto:
	// intentionally generic, anti-enumeration).
	ErrRecoveryCodeInvalid = errors.New("recoverycode: invalid code")
)
