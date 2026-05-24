package domain

import "fmt"

type RoleStatus uint8

const (
	RoleStatusUnspecified RoleStatus = 0
	RoleStatusActive      RoleStatus = 1
	RoleStatusDisabled    RoleStatus = 2
)

func (s RoleStatus) Valid() bool {
	switch s {
	case RoleStatusActive, RoleStatusDisabled:
		return true
	}
	return false
}

func (s RoleStatus) String() string {
	switch s {
	case RoleStatusUnspecified:
		return "UNSPECIFIED"
	case RoleStatusActive:
		return "ACTIVE"
	case RoleStatusDisabled:
		return "DISABLED"
	}
	return fmt.Sprintf("RoleStatus(%d)", s)
}
