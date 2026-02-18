package database

import (
	"github.com/icinga/icinga-go-library/config"
	"github.com/pkg/errors"
)

// Config defines database client configuration.
type Config struct {
	Type       string     `yaml:"type" env:"TYPE" default:"mysql"`
	Host       string     `yaml:"host" env:"HOST"`
	Port       int        `yaml:"port" env:"PORT"`
	Database   string     `yaml:"database" env:"DATABASE"`
	User       string     `yaml:"user" env:"USER"`
	Password   string     `yaml:"password" env:"PASSWORD,unset"` // #nosec G117 -- exported password field
	TlsOptions config.TLS `yaml:",inline"`
	Options    Options    `yaml:"options" envPrefix:"OPTIONS_"`
}

// Validate checks constraints in the supplied database configuration and returns an error if they are violated.
func (c *Config) Validate() error {
	switch c.Type {
	case "mysql", "pgsql":
	default:
		return unknownDbType(c.Type)
	}

	if c.Host == "" {
		return errors.New("database host missing")
	}

	if c.User == "" {
		return errors.New("database user missing")
	}

	if c.Database == "" {
		return errors.New("database name missing")
	}

	return c.Options.Validate()
}

func unknownDbType(t string) error {
	return errors.Errorf(`unknown database type %q, must be one of: "mysql", "pgsql"`, t)
}
