package config

import "fmt"

// Env identifies the deployment profile. Drives profile-specific behavior
// (e.g. reflection on/off, log verbosity).
type Env string

const (
	EnvLocal Env = "local"
	EnvDev   Env = "dev"
	EnvProd  Env = "prod"
)

func (e *Env) Valid() bool {
	switch *e {
	case EnvLocal, EnvDev, EnvProd:
		return true
	}
	return false
}

func (e *Env) validate() error {
	if !e.Valid() {
		return fmt.Errorf("env: %v is not one of %s/%s/%s", e, EnvLocal, EnvDev, EnvProd)
	}
	return nil
}
