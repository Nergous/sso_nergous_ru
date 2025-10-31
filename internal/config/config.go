package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env           string        `yaml:"env" env-default:"local"`
	DB            Database      `yaml:"database"`
	TokenTTL      time.Duration `yaml:"token_ttl" env-required:"true"`
	RefreshTTL    time.Duration `yaml:"refresh_ttl" env-required:"true"`
	GRPC          GRPCConfig    `yaml:"grpc"`
	DefaultSecret string        `yaml:"default_secret" env-required:"true"`
}

type Database struct {
	Host       string `yaml:"host" env:"HOST" env-default:"localhost"`
	Port       int    `yaml:"port" env:"PORT" env-required:"true"`
	UsernameDB string `yaml:"username-db" env:"USERNAMEDB" env-required:"true"`
	Password   string `yaml:"password" env:"PASSWORD"`
	DBName     string `yaml:"dbname" env:"DBNAME" env-default:"games"`
}

type GRPCConfig struct {
	Port    int           `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

func MustLoad() *Config {
	path := fetchConfigPath()
	if path == "" {
		panic("config path is empty")
	}
	return MustLoadByPath(path)
}

func MustLoadByPath(configPath string) *Config {
	path := configPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic("config file does not exist")
	}

	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		panic("failed to read config: " + err.Error())
	}

	return &cfg
}

// fetchConfigPath fetches config path from command line flag or env variable
// Priority: flag > env > default
// Default value is empty string
func fetchConfigPath() string {
	var res string

	// --config="path/to/config.yaml"
	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}
	return res
}

func (cfg *Database) GetDSN() string {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.UsernameDB,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)

	log.Print(dsn)

	return dsn
}
