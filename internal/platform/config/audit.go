package config

import (
	"errors"
	"fmt"
)

// AuditConfig controls the audit pipeline. Enabled=false swaps the
// SyncEmitter for a NopEmitter at bootstrap, so use-cases keep their
// audit calls but nothing reaches the audit_events table — handy for
// dev / tests that don't want the round-trip. RetentionDays drives the
// `migrator -cmd audit:purge` default cutoff.
type AuditConfig struct {
	Enabled       bool `yaml:"enabled" env:"AUDIT_ENABLED" env-default:"true"`
	RetentionDays int  `yaml:"retention_days" env:"AUDIT_RETENTION_DAYS" env-default:"365"`
}

func (c *AuditConfig) validate() error {
	var errs []error

	if c.RetentionDays <= 0 {
		errs = append(errs, fmt.Errorf("audit.retention_days: must be > 0"))
	}

	return errors.Join(errs...)
}
