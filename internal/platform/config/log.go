package config

import (
	"errors"
	"fmt"
)

// LogConfig controls slog: severity threshold, output format, sink, and whether
// source code locations are attached to records.
type LogConfig struct {
	Level     string `yaml:"level" env:"LOG_LEVEL" env-default:"info"`
	Format    string `yaml:"format" env:"LOG_FORMAT" env-default:"text"`
	Sink      string `yaml:"sink" env:"LOG_SINK" env-default:"stdout"`
	Path      string `yaml:"path" env:"LOG_PATH" env-default:""`
	AddSource bool   `yaml:"add_source" env:"LOG_ADD_SOURCE" env-default:"false"`
}

func (c *LogConfig) validate() error {
	var errs []error
	switch c.Level {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("log.level: %q is not one of debug/info/warn/error", c.Level))
	}
	switch c.Format {
	case "text", "json":
	default:
		errs = append(errs, fmt.Errorf("log.format: %q is not one of text/json", c.Format))
	}
	switch c.Sink {
	case "stdout", "file":
	default:
		errs = append(errs, fmt.Errorf("log.sink: %q is not one of stdout/file", c.Sink))
	}
	if c.Sink == "file" && c.Path == "" {
		errs = append(errs, fmt.Errorf("log.path: required when log.sink=file"))
	}
	return errors.Join(errs...)
}
