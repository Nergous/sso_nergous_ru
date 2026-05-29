package domain

import "fmt"

// AppStatus is the lifecycle state of an application record.
//
// Numeric values are stable and mirror sso.app.v1.AppStatus minus the
// proto-only UNSPECIFIED sentinel. They are also the on-wire
// representation of the `status` column in the apps table — do not
// renumber.
type AppStatus uint8

const (
	AppStatusActive      AppStatus = 1
	AppStatusDisabled    AppStatus = 2
	AppStatusMaintenance AppStatus = 3
)

func (s AppStatus) Valid() bool {
	switch s {
	case AppStatusActive, AppStatusDisabled, AppStatusMaintenance:
		return true
	}
	return false
}

func (s AppStatus) String() string {
	switch s {
	case AppStatusActive:
		return "ACTIVE"
	case AppStatusDisabled:
		return "DISABLED"
	case AppStatusMaintenance:
		return "MAINTENANCE"
	}
	return fmt.Sprintf("AppStatus(%d)", s)
}
