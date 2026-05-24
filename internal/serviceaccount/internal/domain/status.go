package domain

import "fmt"

type ServiceAccountStatus uint8

const (
	ServiceAccountActive   ServiceAccountStatus = 1
	ServiceAccountDisabled ServiceAccountStatus = 2
)

func (s ServiceAccountStatus) Valid() bool {
	switch s {
	case ServiceAccountActive, ServiceAccountDisabled:
		return true
	}
	return false
}

func (s ServiceAccountStatus) String() string {
	switch s {
	case ServiceAccountActive:
		return "ACTIVE"
	case ServiceAccountDisabled:
		return "DISABLED"
	}
	return fmt.Sprintf("ServiceAccountStatus(%d)", s)
}
