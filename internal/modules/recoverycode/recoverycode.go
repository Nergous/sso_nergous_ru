// Package recoverycode is the public API of the recoverycode bounded
// context (one-time codes for offline password reset).
//
// External callers interact with the module through these surfaces:
//
//	recoverycode.New(Deps)    wires the module (module.go)
//	recoverycode.Repository   persistence contract (consumed by auth)
//
// The type aliases below let other modules program against
// recoverycode.Batch / recoverycode.BatchID etc. instead of importing
// the internal domain package directly. The internal package stays
// unreachable thanks to Go's "internal/" protection.
package recoverycode

import "sso/internal/modules/recoverycode/internal/domain"

type (
	Batch              = domain.Batch
	BatchID            = domain.BatchID
	UserID             = domain.UserID
	Code               = domain.Code
	NewBatchParams     = domain.NewBatchParams
	RestoreBatchParams = domain.RestoreBatchParams
	Repository         = domain.Repository
)

// ID constructors / parsers re-exported as package-level variables.
var (
	NewBatchID   = domain.NewBatchID
	ParseBatchID = domain.ParseBatchID
	NewBatch     = domain.NewBatch
	RestoreBatch = domain.RestoreBatch
	NewCode      = domain.NewCode
	RestoreCode  = domain.RestoreCode
)

// Sentinel errors. External consumers test for them with errors.Is.
var (
	ErrBatchNotFound      = domain.ErrBatchNotFound
	ErrRecoveryCodeInvalid = domain.ErrRecoveryCodeInvalid
)
