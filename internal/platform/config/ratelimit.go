package config

import (
	"errors"
	"fmt"
	"time"
)

type RateLimitConfig struct {
	Enabled         bool           `yaml:"enabled" env-default:"false"`
	CleanupInterval time.Duration  `yaml:"cleanup_interval" env-default:"5m"`
	Policies        PoliciesConfig `yaml:"policies"`
}

type Policy struct {
	Rps   float64 `yaml:"rps" env-default:"0.5"`
	Burst int     `yaml:"burst" env-default:"30"`
}

type PoliciesConfig struct {
	LoginPerIP            Policy `yaml:"login_per_ip"`
	LoginPerUsername      Policy `yaml:"login_per_username"`
	ResetPerIP            Policy `yaml:"reset_per_ip"`
	ResetPerEmail         Policy `yaml:"reset_per_email"`
	ServiceAuthPerClient  Policy `yaml:"service_auth_per_client"`
	ChangePasswordPerUser Policy `yaml:"change_password_per_user"`
}

func (c *RateLimitConfig) validate() error {
	var errs []error

	if c.CleanupInterval <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.cleanup_interval: must be > 0"))
	}

	if c.Policies.LoginPerIP.Rps <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.login_per_ip.rps: must be > 0"))
	}

	if c.Policies.LoginPerIP.Burst <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.login_per_username.burst: must be > 0"))
	}

	if c.Policies.LoginPerUsername.Rps <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.login_per_username.rps: must be > 0"))
	}

	if c.Policies.LoginPerUsername.Burst <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.login_per_username.burst: must be > 0"))
	}

	if c.Policies.ResetPerIP.Rps <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.reset_per_ip.rps: must be > 0"))
	}

	if c.Policies.ResetPerIP.Burst <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.reset_per_ip.burst: must be > 0"))
	}

	if c.Policies.ResetPerEmail.Rps <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.reset_per_email.rps: must be > 0"))
	}

	if c.Policies.ResetPerEmail.Burst <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.reset_per_email.burst: must be > 0"))
	}

	if c.Policies.ServiceAuthPerClient.Rps <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.service_auth_per_client.rps: must be > 0"))
	}

	if c.Policies.ServiceAuthPerClient.Burst <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.service_auth_per_client.burst: must be > 0"))
	}

	if c.Policies.ChangePasswordPerUser.Rps <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.change_password_per_user.rps: must be > 0"))
	}

	if c.Policies.ChangePasswordPerUser.Burst <= 0 {
		errs = append(errs, fmt.Errorf("ratelimit.policies.change_password_per_user.burst: must be > 0"))
	}

	return errors.Join(errs...)
}
