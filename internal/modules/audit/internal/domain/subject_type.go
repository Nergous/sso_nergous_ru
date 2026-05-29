package domain

// SubjectType mirrors sso.audit.v1.SubjectType. Kept in the domain
// (rather than reusing platform/jwt.SubjectType) so the audit aggregate
// has no dependency on transport-layer code.
//
// Numbering matches the proto enum exactly so a mapper can cast 1-to-1.
type SubjectType uint8

const (
	SubjectTypeUnknown        SubjectType = 0
	SubjectTypeUser           SubjectType = 1
	SubjectTypeRole           SubjectType = 2
	SubjectTypeApp            SubjectType = 3
	SubjectTypeSession        SubjectType = 4
	SubjectTypeRoleAssignment SubjectType = 5
	SubjectTypeServiceAccount SubjectType = 6
)

func (s SubjectType) String() string {
	switch s {
	case SubjectTypeUser:
		return "user"
	case SubjectTypeRole:
		return "role"
	case SubjectTypeApp:
		return "app"
	case SubjectTypeSession:
		return "session"
	case SubjectTypeRoleAssignment:
		return "role_assignment"
	case SubjectTypeServiceAccount:
		return "service_account"
	default:
		return "unknown"
	}
}

func (s SubjectType) IsKnown() bool {
	switch s {
	case SubjectTypeUser,
		SubjectTypeRole,
		SubjectTypeApp,
		SubjectTypeSession,
		SubjectTypeRoleAssignment,
		SubjectTypeServiceAccount:
		return true
	default:
		return false
	}
}
