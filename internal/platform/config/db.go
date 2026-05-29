package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// TLSMode mirrors the values accepted by github.com/go-sql-driver/mysql:
// "false", "true", "skip-verify", "preferred", or a name registered via
// mysql.RegisterTLSConfig. Only non-empty is enforced; the driver validates
// the rest at connect time.
type TLSMode string

type DatabaseConfig struct {
	Driver          string           `yaml:"driver" env:"DB_DRIVER" env-required:"true"`
	Connection      ConnectionConfig `yaml:"connection"`
	Options         OptionsConfig    `yaml:"options"`
	Pool            PoolConfig       `yaml:"pool"`
	ShutdownTimeout time.Duration    `yaml:"shutdown_timeout" env:"DB_SHUTDOWN_TIMEOUT" env-default:"5s"`
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

type PoolConfig struct {
	MaxOpenConns    int           `yaml:"max_open_conns" env:"DB_MAX_OPEN_CONNS" env-default:"10"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" env-default:"5"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" env-default:"5m"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME" env-default:"2m"`
}

// DSN assembles a MySQL/MariaDB DSN: user:pass@tcp(host:port)/db?opts.
// The Driver field is informational; the format is mysql-specific.
func (c *DatabaseConfig) DSN() string {
	addr := net.JoinHostPort(c.Connection.Host, strconv.Itoa(c.Connection.Port))
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s",
		c.Connection.Username,
		string(c.Connection.Password),
		addr,
		c.Connection.DBName,
		c.Options.Query(),
	)
}

// Query renders the "after-?" portion of the DSN.
func (c *OptionsConfig) Query() string {
	parts := []string{
		"timeout=" + c.Timeout.String(),
		"readTimeout=" + c.ReadTimeout.String(),
		"writeTimeout=" + c.WriteTimeout.String(),
		"tls=" + string(c.TLS),
	}
	if c.Params != "" {
		parts = append(parts, c.Params)
	}
	return strings.Join(parts, "&")
}

func (c *DatabaseConfig) validate() error {
	var errs []error
	if c.Driver == "" {
		errs = append(errs, fmt.Errorf("database.driver: required"))
	}
	if c.Connection.Port < 1 || c.Connection.Port > 65535 {
		errs = append(errs, fmt.Errorf("database.connection.port: %d out of range 1..65535", c.Connection.Port))
	}
	if c.Connection.Username == "" {
		errs = append(errs, fmt.Errorf("database.connection.username: required"))
	}
	if c.Connection.DBName == "" {
		errs = append(errs, fmt.Errorf("database.connection.db_name: required"))
	}
	if c.Options.TLS == "" {
		errs = append(errs, fmt.Errorf("database.options.tls: required (e.g. \"false\", \"true\", \"skip-verify\", \"preferred\")"))
	}
	if c.Options.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("database.options.timeout: must be > 0"))
	}
	if c.Options.ReadTimeout <= 0 {
		errs = append(errs, fmt.Errorf("database.options.read_timeout: must be > 0"))
	}
	if c.Options.WriteTimeout <= 0 {
		errs = append(errs, fmt.Errorf("database.options.write_timeout: must be > 0"))
	}
	if c.Pool.MaxOpenConns < 1 {
		errs = append(errs, fmt.Errorf("database.pool.max_open_conns: must be >= 1"))
	}
	if c.Pool.MaxIdleConns < 0 {
		errs = append(errs, fmt.Errorf("database.pool.max_idle_conns: must be >= 0"))
	}
	if c.Pool.MaxIdleConns > c.Pool.MaxOpenConns {
		errs = append(errs, fmt.Errorf("database.pool.max_idle_conns (%d) must be <= max_open_conns (%d)",
			c.Pool.MaxIdleConns, c.Pool.MaxOpenConns))
	}
	if c.Pool.ConnMaxLifetime < 0 {
		errs = append(errs, fmt.Errorf("database.pool.conn_max_lifetime: must be >= 0"))
	}
	if c.Pool.ConnMaxIdleTime < 0 {
		errs = append(errs, fmt.Errorf("database.pool.conn_max_idle_time: must be >= 0"))
	}
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("database.shutdown_timeout: must be > 0"))
	}
	return errors.Join(errs...)
}
