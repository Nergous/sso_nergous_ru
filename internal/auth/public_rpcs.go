package auth

// PublicRPCs lists the fully-qualified method names that the grpcauth
// interceptor MUST allow through without a bearer token. Maps 1:1 to the
// "Public (no access_token)" set documented in the AuthService proto:
//
//   - Register / Login: no token by definition.
//   - Refresh: identifies the caller via the refresh_token itself.
//   - ValidateToken: introspection takes the access_token in the request
//     body, not in metadata; the use-case performs its own session check.
//   - ResetPasswordWithRecoveryCode: forgot-password flow, by design
//     unauthenticated.
//   - AuthenticateServiceAccount: client_credentials grant, the request
//     body IS the credential.
//
// MFA-related RPCs (CompleteMfaChallenge) are intentionally absent: MFA
// was descoped from the v1 surface. RevokeToken / RevokeSession /
// RevokeAllSessions / Logout / ChangePassword / ListSessions /
// GenerateRecoveryCodes all require an authenticated caller, so they
// stay out of this list.
var PublicRPCs = []string{
	"/sso.auth.v1.AuthService/Register",
	"/sso.auth.v1.AuthService/Login",
	"/sso.auth.v1.AuthService/Refresh",
	"/sso.auth.v1.AuthService/ValidateToken",
	"/sso.auth.v1.AuthService/ResetPasswordWithRecoveryCode",
	"/sso.auth.v1.AuthService/AuthenticateServiceAccount",
}
