// Package auth is the public API of the auth bounded context (login,
// refresh, password change, recovery, service-account authentication).
//
// External callers interact with the module through these surfaces:
//
//	auth.New(Deps)    wires the module (module.go)
//	auth.Service      application-layer use-cases (service.go re-exports)
//	auth.PublicRPCs   slice of RPCs that bypass the grpcauth interceptor
//
// auth has no domain aggregates of its own — it orchestrates across
// identity, session, recoverycode, app, serviceaccount. The Input /
// Output type aliases below are the typed contracts of each use-case;
// the gRPC adapter (internal/grpc) converts to and from these.
package auth

import "sso/internal/auth/internal/service"

// Service is the use-case orchestrator. Methods correspond 1-to-1 to
// the AuthService RPCs and are split across files in internal/service.
type Service = service.Service

// Input / Output type aliases.
type (
	RegisterInput                       = service.RegisterInput
	LoginInput                          = service.LoginInput
	LoginOutput                         = service.LoginOutput
	RefreshInput                        = service.RefreshInput
	RefreshOutput                       = service.RefreshOutput
	LogoutInput                         = service.LogoutInput
	ValidateInput                       = service.ValidateInput
	ValidateOutput                      = service.ValidateOutput
	ChangePasswordInput                 = service.ChangePasswordInput
	ChangePasswordOutput                = service.ChangePasswordOutput
	ListSessionsInput                   = service.ListSessionsInput
	ListSessionsOutput                  = service.ListSessionsOutput
	RevokeSessionInput                  = service.RevokeSessionInput
	RevokeAllSessionsInput              = service.RevokeAllSessionsInput
	RevokeTokenInput                    = service.RevokeTokenInput
	GenerateRecoveryCodesInput          = service.GenerateRecoveryCodesInput
	GenerateRecoveryCodesOutput         = service.GenerateRecoveryCodesOutput
	ResetPasswordWithRecoveryCodeInput  = service.ResetPasswordWithRecoveryCodeInput
	ResetPasswordWithRecoveryCodeOutput = service.ResetPasswordWithRecoveryCodeOutput
	AuthenticateServiceAccountInput     = service.AuthenticateServiceAccountInput
	AuthenticateServiceAccountOutput    = service.AuthenticateServiceAccountOutput
)
