package redis

import (
	"time"

	"github.com/icinga/icinga-go-library/config"
	"github.com/pkg/errors"
)

// Options define user configurable Redis options.
type Options struct {
	BlockTimeout        time.Duration `yaml:"block_timeout" env:"BLOCK_TIMEOUT" default:"1s"`
	HMGetCount          int           `yaml:"hmget_count" env:"HMGET_COUNT" default:"4096"`
	HScanCount          int           `yaml:"hscan_count" env:"HSCAN_COUNT" default:"4096"`
	MaxHMGetConnections int           `yaml:"max_hmget_connections" env:"MAX_HMGET_CONNECTIONS" default:"8"`
	Timeout             time.Duration `yaml:"timeout" env:"TIMEOUT" default:"30s"`
	XReadCount          int           `yaml:"xread_count" env:"XREAD_COUNT" default:"4096"`
}

// Validate checks constraints in the supplied Redis options and returns an error if they are violated.
func (o *Options) Validate() error {
	if o.BlockTimeout <= 0 {
		return errors.New("block_timeout must be positive")
	}
	if o.HMGetCount < 1 {
		return errors.New("hmget_count must be at least 1")
	}
	if o.HScanCount < 1 {
		return errors.New("hscan_count must be at least 1")
	}
	if o.MaxHMGetConnections < 1 {
		return errors.New("max_hmget_connections must be at least 1")
	}
	if o.Timeout == 0 {
		return errors.New("timeout cannot be 0. Configure a value greater than zero, or use -1 for no timeout")
	}
	if o.XReadCount < 1 {
		return errors.New("xread_count must be at least 1")
	}

	return nil
}

// Config defines Config client configuration.
type Config struct {
	Host         string     `yaml:"host" env:"HOST"`
	Port         int        `yaml:"port" env:"PORT"`
	Username     string     `yaml:"username" env:"USERNAME"`
	Password     string     `yaml:"password" env:"PASSWORD,unset"` // #nosec G117 -- exported password field
	PasswordFile string     `yaml:"password_file" env:"PASSWORD_FILE"`
	Database     int        `yaml:"database" env:"DATABASE" default:"0"`
	TlsOptions   config.TLS `yaml:",inline"`
	Options      Options    `yaml:"options" envPrefix:"OPTIONS_"`
}

// Validate checks constraints in the supplied Config configuration and returns an error if they are violated.
func (r *Config) Validate() error {
	if r.Host == "" {
		return errors.New("Redis host missing")
	}

	if err := config.LoadPasswordFile(&r.Password, r.PasswordFile); err != nil {
		return err
	}

	if r.Username != "" && r.Password == "" {
		return errors.New("Redis password must be set, if username is provided")
	}

	return r.Options.Validate()
}
