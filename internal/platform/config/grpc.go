package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

type GRPCConfig struct {
	Host              string                `yaml:"host" env:"GRPC_HOST" env-default:"127.0.0.1"`
	Port              int                   `yaml:"port" env:"GRPC_PORT" env-required:"true"`
	ConnectionTimeout time.Duration         `yaml:"connection_timeout" env:"GRPC_CONNECTION_TIMEOUT" env-default:"30s"`
	ShutdownTimeout   time.Duration         `yaml:"shutdown_timeout" env:"GRPC_SHUTDOWN_TIMEOUT" env-default:"30s"`
	MaxRecvMsgSize    int                   `yaml:"max_recv_msg_size" env:"GRPC_MAX_RECV_MSG_SIZE" env-default:"4194304"`
	MaxSendMsgSize    int                   `yaml:"max_send_msg_size" env:"GRPC_MAX_SEND_MSG_SIZE" env-default:"4194304"`
	Keepalive         KeepaliveConfig       `yaml:"keepalive"`
	HealthCheck       GRPCHealthCheckConfig `yaml:"health_check"`
	Reflection        GRPCReflectionConfig  `yaml:"reflection"`
	TLS               GRPCTLSConfig         `yaml:"tls"`
}

// KeepaliveConfig maps to grpc.KeepaliveParams + KeepaliveEnforcementPolicy.
// Time: server pings idle clients every Time; Timeout: ack deadline before
// the conn is closed. MinTime/PermitWithoutStream constrain client pings.
type KeepaliveConfig struct {
	Time                time.Duration `yaml:"time" env:"GRPC_KEEPALIVE_TIME" env-default:"60s"`
	Timeout             time.Duration `yaml:"timeout" env:"GRPC_KEEPALIVE_TIMEOUT" env-default:"20s"`
	MinTime             time.Duration `yaml:"min_time" env:"GRPC_KEEPALIVE_MIN_TIME" env-default:"30s"`
	PermitWithoutStream bool          `yaml:"permit_without_stream" env:"GRPC_KEEPALIVE_PERMIT_WITHOUT_STREAM" env-default:"false"`
}

type GRPCHealthCheckConfig struct {
	Enabled bool `yaml:"enabled" env:"GRPC_HEALTH_CHECK_ENABLED" env-default:"true"`
}

type GRPCReflectionConfig struct {
	Enabled bool `yaml:"enabled" env:"GRPC_REFLECTION_ENABLED" env-default:"false"`
}

type GRPCTLSConfig struct {
	Enabled  bool   `yaml:"enabled" env:"GRPC_TLS_ENABLED" env-default:"false"`
	CertPath string `yaml:"cert_path" env:"GRPC_TLS_CERT_PATH" env-default:""`
	KeyPath  string `yaml:"key_path" env:"GRPC_TLS_KEY_PATH" env-default:""`
}

func (c *GRPCConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func (c *GRPCConfig) validate() error {
	var errs []error
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("grpc.port: %d out of range 1..65535", c.Port))
	}
	if c.ConnectionTimeout <= 0 {
		errs = append(errs, fmt.Errorf("grpc.connection_timeout: must be > 0"))
	}
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("grpc.shutdown_timeout: must be > 0"))
	}
	if c.MaxRecvMsgSize <= 0 {
		errs = append(errs, fmt.Errorf("grpc.max_recv_msg_size: must be > 0"))
	}
	if c.MaxSendMsgSize <= 0 {
		errs = append(errs, fmt.Errorf("grpc.max_send_msg_size: must be > 0"))
	}
	if c.Keepalive.Time <= 0 {
		errs = append(errs, fmt.Errorf("grpc.keepalive.time: must be > 0"))
	}
	if c.Keepalive.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("grpc.keepalive.timeout: must be > 0"))
	}
	if c.Keepalive.MinTime <= 0 {
		errs = append(errs, fmt.Errorf("grpc.keepalive.min_time: must be > 0"))
	}
	if c.TLS.Enabled {
		if c.TLS.CertPath == "" {
			errs = append(errs, fmt.Errorf("grpc.tls.cert_path: required when grpc.tls.enabled=true"))
		}
		if c.TLS.KeyPath == "" {
			errs = append(errs, fmt.Errorf("grpc.tls.key_path: required when grpc.tls.enabled=true"))
		}
	}
	return errors.Join(errs...)
}
