package domain

import "fmt"

// UserStatus is the lifecycle state of an identity record.
//
// Numeric values are stable and mirror sso.identity.v1.UserStatus minus the
// proto-only UNSPECIFIED sentinel. They are also the on-wire representation
// of the `status` column in the users table — do not renumber.
type UserStatus uint8

const (
	UserStatusActive  UserStatus = 1
	UserStatusBlocked UserStatus = 2
	UserStatusDeleted UserStatus = 3
)

func (s UserStatus) Valid() bool {
	switch s {
	case UserStatusActive, UserStatusBlocked, UserStatusDeleted:
		return true
	}
	return false
}

func (s UserStatus) String() string {
	switch s {
	case UserStatusActive:
		return "ACTIVE"
	case UserStatusBlocked:
		return "BLOCKED"
	case UserStatusDeleted:
		return "DELETED"
	}
	return fmt.Sprintf("UserStatus(%d)", s)
}
