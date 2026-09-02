package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// Config is process configuration. Sources: config.json < environment.
// Env names follow nested keys: app.port → APP_PORT, database.host → DATABASE_HOST.
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
}

type AppConfig struct {
	Port int `mapstructure:"port"`
}

type LogConfig struct {
	Level slog.Level `mapstructure:"level"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`

	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

func Load() (Config, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetConfigName("config")
	v.SetConfigType("json")
	v.AddConfigPath(".")
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.TextUnmarshallerHookFunc(),
	))); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if !v.IsSet("database.max_idle_conns") {
		cfg.Database.MaxIdleConns = cfg.Database.MaxOpenConns
	}

	cfg.normalize()
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) normalize() {
	c.Database.Host = strings.TrimSpace(c.Database.Host)
	c.Database.User = strings.TrimSpace(c.Database.User)
	c.Database.Password = strings.TrimSpace(c.Database.Password)
	c.Database.Name = strings.TrimSpace(c.Database.Name)
	c.Database.SSLMode = strings.TrimSpace(c.Database.SSLMode)
}

func (c Config) validate() error {
	if c.App.Port < 1 || c.App.Port > 65535 {
		return fmt.Errorf("APP_PORT must be between 1 and 65535, got %d", c.App.Port)
	}
	if c.Database.Host == "" {
		return fmt.Errorf("DATABASE_HOST is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("DATABASE_USER is required")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("DATABASE_NAME is required")
	}
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("DATABASE_PORT must be between 1 and 65535, got %d", c.Database.Port)
	}
	if c.Database.MaxOpenConns < 1 {
		return fmt.Errorf("DATABASE_MAX_OPEN_CONNS must be >= 1, got %d", c.Database.MaxOpenConns)
	}
	if c.Database.MaxIdleConns < 0 {
		return fmt.Errorf("DATABASE_MAX_IDLE_CONNS must be >= 0, got %d", c.Database.MaxIdleConns)
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("DATABASE_MAX_IDLE_CONNS (%d) cannot exceed DATABASE_MAX_OPEN_CONNS (%d)", c.Database.MaxIdleConns, c.Database.MaxOpenConns)
	}
	if c.Database.ConnMaxLifetime <= 0 {
		return fmt.Errorf("DATABASE_CONN_MAX_LIFETIME must be > 0, got %s", c.Database.ConnMaxLifetime)
	}
	if c.Database.ConnMaxIdleTime <= 0 {
		return fmt.Errorf("DATABASE_CONN_MAX_IDLE_TIME must be > 0, got %s", c.Database.ConnMaxIdleTime)
	}
	return nil
}

// DSN builds a postgres URL. Prefer this over concatenating; passwords can contain reserved characters.
func (d DatabaseConfig) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:   "/" + d.Name,
	}
	if d.Password != "" {
		u.User = url.UserPassword(d.User, d.Password)
	} else {
		u.User = url.User(d.User)
	}
	q := url.Values{}
	q.Set("sslmode", d.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}
