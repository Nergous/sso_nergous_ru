package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

// Secret hides its value in logs and JSON. The plaintext is still available
// via the underlying string conversion when constructing DSNs / credentials.
type Secret string

func (s Secret) String() string {
	if s == "" {
		return ""
	}
	return "***"
}

func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"***"`), nil }
func (Secret) MarshalYAML() (any, error)    { return "***", nil }

type Config struct {
	Env       Env             `yaml:"env" env:"APP_ENV" env-required:"true"`
	Log       LogConfig       `yaml:"log"`
	GRPC      GRPCConfig      `yaml:"grpc"`
	HTTP      HTTPConfig      `yaml:"http"`
	Database  DatabaseConfig  `yaml:"database"`
	Auth      AuthConfig      `yaml:"auth"`
	Audit     AuditConfig     `yaml:"audit"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

const EnvConfigPath = "CONFIG_PATH"

// FetchPath resolves the config path from (in order): the given flag value,
// then $CONFIG_PATH. Returns an error if both are empty.
func FetchPath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if v := os.Getenv(EnvConfigPath); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("config path not set: pass -cp/--configPath or set %s", EnvConfigPath)
}

func Load(configPath string) (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("read config %q: %w", configPath, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

func MustLoad(configPath string) *Config {
	cfg, err := Load(configPath)
	if err != nil {
		panic(err)
	}
	return cfg
}

// Help returns a human-readable summary of every configurable field and its
// env override, generated from struct tags by cleanenv.
func Help() (string, error) {
	return cleanenv.GetDescription(&Config{}, nil)
}

func (c *Config) Validate() error {
	return errors.Join(
		c.Env.validate(),
		c.Log.validate(),
		c.GRPC.validate(),
		c.HTTP.validate(),
		c.Database.validate(),
		c.Auth.validate(),
		c.Audit.validate(),
		c.RateLimit.validate(),
	)
}
