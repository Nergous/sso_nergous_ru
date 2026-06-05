package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// HTTPConfig controls the public HTTP listener that fronts the gRPC server via
// grpc-gateway. When Enabled=false the HTTP listener is not started and the
// process serves gRPC only. CORS and TLS are independent subsections.
type HTTPConfig struct {
	Enabled           bool          `yaml:"enabled" env:"HTTP_ENABLED" env-default:"true"`
	Host              string        `yaml:"host" env:"HTTP_HOST" env-default:"127.0.0.1"`
	Port              int           `yaml:"port" env:"HTTP_PORT" env-default:"8080"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout" env:"HTTP_SHUTDOWN_TIMEOUT" env-default:"30s"`
	WriteTimeout      time.Duration `yaml:"write_timeout" env:"HTTP_WRITE_TIMEOUT" env-default:"30s"`
	ReadTimeout       time.Duration `yaml:"read_timeout" env:"HTTP_READ_TIMEOUT" env-default:"30s"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout" env:"HTTP_READ_HEADER_TIMEOUT" env-default:"30s"`
	IdleTimeout       time.Duration `yaml:"idle_timeout" env:"HTTP_IDLE_TIMEOUT" env-default:"60s"`
	CORS              CORSConfig    `yaml:"cors"`
	TLS               HTTPTLSConfig `yaml:"tls"`
}

// CORSConfig configures the HTTP CORS middleware. AllowedOrigins entries are
// matched exactly against the request Origin; the wildcard "*" disables the
// Access-Control-Allow-Credentials response (browsers ignore credentials on
// wildcard origins).
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins" env:"HTTP_CORS_ALLOWED_ORIGINS" env-separator:","`
}

type HTTPTLSConfig struct {
	Enabled  bool   `yaml:"enabled" env:"HTTP_TLS_ENABLED" env-default:"false"`
	CertPath string `yaml:"cert_path" env:"HTTP_TLS_CERT_PATH" env-default:""`
	KeyPath  string `yaml:"key_path" env:"HTTP_TLS_KEY_PATH" env-default:""`
}

func (c *HTTPConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func (c *HTTPConfig) validate() error {
	if !c.Enabled {
		return nil
	}
	var errs []error
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("http.port: %d out of range 1..65535", c.Port))
	}
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("http.shutdown_timeout: must be > 0"))
	}
	if c.TLS.Enabled {
		if c.TLS.CertPath == "" {
			errs = append(errs, fmt.Errorf("http.tls.cert_path: required when http.tls.enabled=true"))
		}
		if c.TLS.KeyPath == "" {
			errs = append(errs, fmt.Errorf("http.tls.key_path: required when http.tls.enabled=true"))
		}
	}
	for i, o := range c.CORS.AllowedOrigins {
		if strings.TrimSpace(o) == "" {
			errs = append(errs, fmt.Errorf("http.cors.allowed_origins[%d]: empty entry", i))
		}
	}
	return errors.Join(errs...)
}
