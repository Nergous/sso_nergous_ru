package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type AuthConfig struct {
	JWT     JWTConfig     `yaml:"jwt"`
	Session SessionConfig `yaml:"session"`
	Bcrypt  BcryptConfig  `yaml:"bcrypt"`
	Lockout LockoutConfig `yaml:"lockout"`
}

type JWTConfig struct {
	PrivateKeyPath string        `yaml:"private_key_path" env:"JWT_PRIVATE_KEY_PATH" env-required:"true"`
	PublicKeyPath  string        `yaml:"public_key_path"  env:"JWT_PUBLIC_KEY_PATH"  env-required:"true"`
	Issuer         string        `yaml:"issuer"           env:"JWT_ISSUER"           env-default:"sso"`
	AccessTTL      time.Duration `yaml:"access_ttl"       env:"JWT_ACCESS_TTL"       env-default:"15m"`
}

type SessionConfig struct {
	RefreshTTL         time.Duration `yaml:"refresh_ttl" env:"SESSION_REFRESH_TTL" env-default:"720h"`                   // 30d hard cap
	RefreshRotationTTL time.Duration `yaml:"refresh_rotation_ttl" env:"SESSION_REFRESH_ROTATION_TTL" env-default:"168h"` // 7d sliding window
}

type BcryptConfig struct {
	Cost int `yaml:"cost" env:"BCRYPT_COST" env-default:"12"`
}

type LockoutConfig struct {
	Threshold int           `yaml:"threshold" env:"LOCKOUT_THRESHOLD" env-default:"5"`
	Duration  time.Duration `yaml:"duration"  env:"LOCKOUT_DURATION"  env-default:"15m"`
}

func (c *AuthConfig) validate() error {
	var errs []error
	if c.JWT.AccessTTL <= 0 {
		errs = append(errs, fmt.Errorf("auth.jwt.access_ttl: must be > 0"))
	}
	if c.Session.RefreshTTL <= 0 {
		errs = append(errs, fmt.Errorf("auth.session.refresh_ttl: must be > 0"))
	}
	if c.Session.RefreshRotationTTL <= 0 {
		errs = append(errs, fmt.Errorf("auth.session.refresh_rotation_ttl: must be > 0"))
	}

	if c.Session.RefreshRotationTTL > c.Session.RefreshTTL {
		errs = append(errs, fmt.Errorf("auth.session.refresh_rotation_ttl: must be <= auth.session.refresh_ttl"))
	}

	if c.Bcrypt.Cost < 4 || c.Bcrypt.Cost > 31 {
		errs = append(errs, fmt.Errorf("auth.bcrypt.cost: must be in range 4..31"))
	}

	if c.JWT.Issuer == "" {
		errs = append(errs, fmt.Errorf("auth.jwt.issuer: required"))
	}

	if _, err := os.Stat(c.JWT.PrivateKeyPath); err != nil {
		errs = append(errs, fmt.Errorf("auth.jwt.private_key_path %q: %w", c.JWT.PrivateKeyPath, err))
	}

	if c.Lockout.Threshold < 0 {
		errs = append(errs, fmt.Errorf("auth.lockout.threshold: must be >= 0"))
	}
	if c.Lockout.Threshold > 0 && c.Lockout.Duration <= 0 {
		errs = append(errs, fmt.Errorf("auth.lockout.duration: must be > 0"))
	}

	return errors.Join(errs...)
}
