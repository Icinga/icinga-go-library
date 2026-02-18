package source

import (
	"github.com/icinga/icinga-go-library/config"
)

// Config defines all configuration for the Icinga Notifications API Client.
type Config struct {
	// Url points to the Icinga Notifications API, e.g., http://localhost:5680
	Url string `yaml:"url" env:"URL"`

	// Username is the API user for the Icinga Notifications API.
	Username string `yaml:"username" env:"USERNAME"`

	// Password is the API user's password for the Icinga Notifications API.
	Password     string `yaml:"password" env:"PASSWORD,unset"` // #nosec G117 -- exported password field
	PasswordFile string `yaml:"password_file" env:"PASSWORD_FILE"`
}

// Validate the configuration, implements config.Validator.
func (c *Config) Validate() error {
	if err := config.LoadPasswordFile(&c.Password, c.PasswordFile); err != nil {
		return err
	}

	return nil
}
