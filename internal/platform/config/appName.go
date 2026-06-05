package config

import "fmt"

type AppName string

func (a AppName) String() string {
	return string(a)
}

func (a AppName) IsEmpty() bool {
	if a == "" {
		return true
	}
	return false
}

func (a AppName) validate() error {
	if a.IsEmpty() {
		return fmt.Errorf("app_name: required")
	}
	return nil
}
