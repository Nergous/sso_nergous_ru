package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Env identifies the deployment profile. Drives profile-specific behavior
// (e.g. reflection on/off, log verbosity).
type Env string

const (
	EnvLocal Env = "local"
	EnvDev   Env = "dev"
	EnvProd  Env = "prod"
)

func (e Env) Valid() bool {
	switch e {
	case EnvLocal, EnvDev, EnvProd:
		return true
	}
	return false
}

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

// TLSMode mirrors the values accepted by github.com/go-sql-driver/mysql:
// "false", "true", "skip-verify", "preferred", or a name registered via
// mysql.RegisterTLSConfig. Only non-empty is enforced; the driver validates
// the rest at connect time.
type TLSMode string

type Config struct {
	Env      Env            `yaml:"env" env:"APP_ENV" env-required:"true"`
	Log      LogConfig      `yaml:"log"`
	GRPC     GRPCConfig     `yaml:"grpc"`
	HTTP     HTTPConfig     `yaml:"http"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Audit    AuditConfig    `yaml:"audit"`
}

// LogConfig controls slog: severity threshold, output format, sink, and whether
// source code locations are attached to records.
type LogConfig struct {
	Level     string `yaml:"level" env:"LOG_LEVEL" env-default:"info"`
	Format    string `yaml:"format" env:"LOG_FORMAT" env-default:"text"`
	Sink      string `yaml:"sink" env:"LOG_SINK" env-default:"stdout"`
	Path      string `yaml:"path" env:"LOG_PATH" env-default:""`
	AddSource bool   `yaml:"add_source" env:"LOG_ADD_SOURCE" env-default:"false"`
}

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

func (c GRPCConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
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

// HTTPConfig controls the public HTTP listener that fronts the gRPC server via
// grpc-gateway. When Enabled=false the HTTP listener is not started and the
// process serves gRPC only. CORS and TLS are independent subsections.
type HTTPConfig struct {
	Enabled         bool          `yaml:"enabled" env:"HTTP_ENABLED" env-default:"true"`
	Host            string        `yaml:"host" env:"HTTP_HOST" env-default:"127.0.0.1"`
	Port            int           `yaml:"port" env:"HTTP_PORT" env-default:"8080"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"HTTP_SHUTDOWN_TIMEOUT" env-default:"30s"`
	CORS            CORSConfig    `yaml:"cors"`
	TLS             HTTPTLSConfig `yaml:"tls"`
}

func (h HTTPConfig) Address() string {
	return net.JoinHostPort(h.Host, strconv.Itoa(h.Port))
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

type DatabaseConfig struct {
	Driver          string           `yaml:"driver" env:"DB_DRIVER" env-required:"true"`
	Connection      ConnectionConfig `yaml:"connection"`
	Options         OptionsConfig    `yaml:"options"`
	Pool            PoolConfig       `yaml:"pool"`
	ShutdownTimeout time.Duration    `yaml:"shutdown_timeout" env:"DB_SHUTDOWN_TIMEOUT" env-default:"5s"`
}

// DSN assembles a MySQL/MariaDB DSN: user:pass@tcp(host:port)/db?opts.
// The Driver field is informational; the format is mysql-specific.
func (d DatabaseConfig) DSN() string {
	addr := net.JoinHostPort(d.Connection.Host, strconv.Itoa(d.Connection.Port))
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s",
		d.Connection.Username,
		string(d.Connection.Password),
		addr,
		d.Connection.DBName,
		d.Options.Query(),
	)
}

type ConnectionConfig struct {
	Host     string `yaml:"host" env:"DB_HOST" env-default:"127.0.0.1"`
	Port     int    `yaml:"port" env:"DB_PORT" env-required:"true"`
	Username string `yaml:"username" env:"DB_USERNAME" env-required:"true"`
	Password Secret `yaml:"password" env:"DB_PASSWORD" env-default:""`
	DBName   string `yaml:"db_name" env:"DB_NAME" env-required:"true"`
}

type OptionsConfig struct {
	Timeout      time.Duration `yaml:"timeout" env:"DB_TIMEOUT" env-default:"5s"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env:"DB_READ_TIMEOUT" env-default:"30s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"DB_WRITE_TIMEOUT" env-default:"30s"`
	TLS          TLSMode       `yaml:"tls" env:"DB_TLS" env-default:"false"`
	Params       string        `yaml:"params" env:"DB_PARAMS" env-default:""`
}

// Query renders the "after-?" portion of the DSN.
func (o OptionsConfig) Query() string {
	parts := []string{
		"timeout=" + o.Timeout.String(),
		"readTimeout=" + o.ReadTimeout.String(),
		"writeTimeout=" + o.WriteTimeout.String(),
		"tls=" + string(o.TLS),
	}
	if o.Params != "" {
		parts = append(parts, o.Params)
	}
	return strings.Join(parts, "&")
}

type PoolConfig struct {
	MaxOpenConns    int           `yaml:"max_open_conns" env:"DB_MAX_OPEN_CONNS" env-default:"10"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" env-default:"5"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" env-default:"5m"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME" env-default:"2m"`
}

type AuthConfig struct {
	JWT     JWTConfig     `yaml:"jwt"`
	Session SessionConfig `yaml:"session"`
	Bcrypt  BcryptConfig  `yaml:"bcrypt"`
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

// AuditConfig controls the audit pipeline. Enabled=false swaps the
// SyncEmitter for a NopEmitter at bootstrap, so use-cases keep their
// audit calls but nothing reaches the audit_events table — handy for
// dev / tests that don't want the round-trip. RetentionDays drives the
// `migrator -cmd audit:purge` default cutoff.
type AuditConfig struct {
	Enabled       bool `yaml:"enabled" env:"AUDIT_ENABLED" env-default:"true"`
	RetentionDays int  `yaml:"retention_days" env:"AUDIT_RETENTION_DAYS" env-default:"365"`
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
		c.validateEnv(),
		c.Log.validate(),
		c.GRPC.validate(),
		c.HTTP.validate(),
		c.Database.validate(),
		c.Auth.validate(),
	)
}

func (c *Config) validateEnv() error {
	if !c.Env.Valid() {
		return fmt.Errorf("env: %q is not one of %s/%s/%s", c.Env, EnvLocal, EnvDev, EnvProd)
	}
	return nil
}

func (l LogConfig) validate() error {
	var errs []error
	switch l.Level {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("log.level: %q is not one of debug/info/warn/error", l.Level))
	}
	switch l.Format {
	case "text", "json":
	default:
		errs = append(errs, fmt.Errorf("log.format: %q is not one of text/json", l.Format))
	}
	switch l.Sink {
	case "stdout", "file":
	default:
		errs = append(errs, fmt.Errorf("log.sink: %q is not one of stdout/file", l.Sink))
	}
	if l.Sink == "file" && l.Path == "" {
		errs = append(errs, fmt.Errorf("log.path: required when log.sink=file"))
	}
	return errors.Join(errs...)
}

func (g GRPCConfig) validate() error {
	var errs []error
	if g.Port < 1 || g.Port > 65535 {
		errs = append(errs, fmt.Errorf("grpc.port: %d out of range 1..65535", g.Port))
	}
	if g.ConnectionTimeout <= 0 {
		errs = append(errs, fmt.Errorf("grpc.connection_timeout: must be > 0"))
	}
	if g.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("grpc.shutdown_timeout: must be > 0"))
	}
	if g.MaxRecvMsgSize <= 0 {
		errs = append(errs, fmt.Errorf("grpc.max_recv_msg_size: must be > 0"))
	}
	if g.MaxSendMsgSize <= 0 {
		errs = append(errs, fmt.Errorf("grpc.max_send_msg_size: must be > 0"))
	}
	if g.Keepalive.Time <= 0 {
		errs = append(errs, fmt.Errorf("grpc.keepalive.time: must be > 0"))
	}
	if g.Keepalive.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("grpc.keepalive.timeout: must be > 0"))
	}
	if g.Keepalive.MinTime <= 0 {
		errs = append(errs, fmt.Errorf("grpc.keepalive.min_time: must be > 0"))
	}
	if g.TLS.Enabled {
		if g.TLS.CertPath == "" {
			errs = append(errs, fmt.Errorf("grpc.tls.cert_path: required when grpc.tls.enabled=true"))
		}
		if g.TLS.KeyPath == "" {
			errs = append(errs, fmt.Errorf("grpc.tls.key_path: required when grpc.tls.enabled=true"))
		}
	}
	return errors.Join(errs...)
}

func (h HTTPConfig) validate() error {
	if !h.Enabled {
		return nil
	}
	var errs []error
	if h.Port < 1 || h.Port > 65535 {
		errs = append(errs, fmt.Errorf("http.port: %d out of range 1..65535", h.Port))
	}
	if h.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("http.shutdown_timeout: must be > 0"))
	}
	if h.TLS.Enabled {
		if h.TLS.CertPath == "" {
			errs = append(errs, fmt.Errorf("http.tls.cert_path: required when http.tls.enabled=true"))
		}
		if h.TLS.KeyPath == "" {
			errs = append(errs, fmt.Errorf("http.tls.key_path: required when http.tls.enabled=true"))
		}
	}
	for i, o := range h.CORS.AllowedOrigins {
		if strings.TrimSpace(o) == "" {
			errs = append(errs, fmt.Errorf("http.cors.allowed_origins[%d]: empty entry", i))
		}
	}
	return errors.Join(errs...)
}

func (d DatabaseConfig) validate() error {
	var errs []error
	if d.Driver == "" {
		errs = append(errs, fmt.Errorf("database.driver: required"))
	}
	if d.Connection.Port < 1 || d.Connection.Port > 65535 {
		errs = append(errs, fmt.Errorf("database.connection.port: %d out of range 1..65535", d.Connection.Port))
	}
	if d.Connection.Username == "" {
		errs = append(errs, fmt.Errorf("database.connection.username: required"))
	}
	if d.Connection.DBName == "" {
		errs = append(errs, fmt.Errorf("database.connection.db_name: required"))
	}
	if d.Options.TLS == "" {
		errs = append(errs, fmt.Errorf("database.options.tls: required (e.g. \"false\", \"true\", \"skip-verify\", \"preferred\")"))
	}
	if d.Options.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("database.options.timeout: must be > 0"))
	}
	if d.Options.ReadTimeout <= 0 {
		errs = append(errs, fmt.Errorf("database.options.read_timeout: must be > 0"))
	}
	if d.Options.WriteTimeout <= 0 {
		errs = append(errs, fmt.Errorf("database.options.write_timeout: must be > 0"))
	}
	if d.Pool.MaxOpenConns < 1 {
		errs = append(errs, fmt.Errorf("database.pool.max_open_conns: must be >= 1"))
	}
	if d.Pool.MaxIdleConns < 0 {
		errs = append(errs, fmt.Errorf("database.pool.max_idle_conns: must be >= 0"))
	}
	if d.Pool.MaxIdleConns > d.Pool.MaxOpenConns {
		errs = append(errs, fmt.Errorf("database.pool.max_idle_conns (%d) must be <= max_open_conns (%d)",
			d.Pool.MaxIdleConns, d.Pool.MaxOpenConns))
	}
	if d.Pool.ConnMaxLifetime < 0 {
		errs = append(errs, fmt.Errorf("database.pool.conn_max_lifetime: must be >= 0"))
	}
	if d.Pool.ConnMaxIdleTime < 0 {
		errs = append(errs, fmt.Errorf("database.pool.conn_max_idle_time: must be >= 0"))
	}
	if d.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("database.shutdown_timeout: must be > 0"))
	}
	return errors.Join(errs...)
}

func (a AuthConfig) validate() error {
	var errs []error
	if a.JWT.AccessTTL <= 0 {
		errs = append(errs, fmt.Errorf("auth.jwt.access_ttl: must be > 0"))
	}
	if a.Session.RefreshTTL <= 0 {
		errs = append(errs, fmt.Errorf("auth.session.refresh_ttl: must be > 0"))
	}
	if a.Session.RefreshRotationTTL <= 0 {
		errs = append(errs, fmt.Errorf("auth.session.refresh_rotation_ttl: must be > 0"))
	}

	if a.Session.RefreshRotationTTL > a.Session.RefreshTTL {
		errs = append(errs, fmt.Errorf("auth.session.refresh_rotation_ttl: must be <= auth.session.refresh_ttl"))
	}

	if a.Bcrypt.Cost < 4 || a.Bcrypt.Cost > 31 {
		errs = append(errs, fmt.Errorf("auth.bcrypt.cost: must be in range 4..31"))
	}

	if a.JWT.Issuer == "" {
		errs = append(errs, fmt.Errorf("auth.jwt.issuer: required"))
	}

	if _, err := os.Stat(a.JWT.PrivateKeyPath); err != nil {
		errs = append(errs, fmt.Errorf("auth.jwt.private_key_path %q: %w", a.JWT.PrivateKeyPath, err))
	}

	return errors.Join(errs...)
}
