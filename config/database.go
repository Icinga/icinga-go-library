package config

import (
	"fmt"
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"go.uber.org/zap"
)

// Database defines database client configuration.
type Database struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	Database       string `yaml:"database"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	MaxConnections int    `yaml:"max_connections" default:"16"`
}

// Open prepares the DSN string and driver configuration,
// calls sqlx.Open, but returns *icingadb.DB.
func (d *Database) Open(logger *zap.SugaredLogger) (*icingadb.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s",
		d.User, d.Password, d.Host, d.Port, d.Database)

	db, err := sqlx.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxIdleConns(d.MaxConnections / 3)
	db.SetMaxOpenConns(d.MaxConnections)

	db.Mapper = reflectx.NewMapperFunc("db", func(s string) string {
		return utils.Key(s, '_')
	})

	return icingadb.NewDb(db, logger), nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (d *Database) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := defaults.Set(d); err != nil {
		return err
	}
	// Prevent recursion.
	type self Database
	if err := unmarshal((*self)(d)); err != nil {
		return err
	}

	return nil
}