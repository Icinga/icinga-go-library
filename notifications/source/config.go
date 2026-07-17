package source

import (
	"net/url"

	"github.com/icinga/icinga-go-library/config"
	"github.com/pkg/errors"
)

const (
	// Supported URI schemes in Config.Url.
	schemeHttp  = "http"
	schemeHttps = "https"
	schemeUnix  = "unix"
)

// Config defines all configuration for the Icinga Notifications API Client.
type Config struct {
	// Url of the Icinga Notifications API.
	//
	// A transport is chosen based on the URI scheme:
	//   - http: Unencrypted HTTP connection. Requires Username and Password/PasswordFile to be set.
	//     For example: http://example.com:5680
	//   - https: HTTPS connection. Either Username and Password/PasswordFile or TlsOptions.{Cert,Key} are required.
	//     For example: https://example.com:5680
	//   - unix: HTTP connection over a Unix Domain Socket. Authentication is based on the operating system user.
	//     So, Username or Password/PasswordFile must not be set. For example: unix:///path/to/socket
	Url string `yaml:"url" env:"URL"`

	// Username is the API user for the Icinga Notifications API.
	//
	// Based on the Config.Url scheme, Username and Password/PasswordFile are either required, allowed, or forbidden.
	Username string `yaml:"username" env:"USERNAME"`

	// Password is the API user's password for the Icinga Notifications API.
	Password     string `yaml:"password" env:"PASSWORD,unset"` // #nosec G117 -- exported password field
	PasswordFile string `yaml:"password_file" env:"PASSWORD_FILE"`

	// TlsOptions are relevant for the "https" Url scheme.
	TlsOptions config.TLS `yaml:",inline"`

	// DefaultRelations to always resolve and include in the events submitted to Icinga Notifications.
	DefaultRelations []string `yaml:"default_relations" env:"DEFAULT_RELATIONS"`
}

// Validate the configuration, implements config.Validator.
func (c *Config) Validate() error {
	if c.Url == "" {
		// Validate an empty, unconfigured config, such as the commented out default in Icinga DB.
		return nil
	}

	u, err := url.Parse(c.Url)
	if err != nil {
		return errors.Wrap(err, "cannot parse notifications configuration URL")
	}

	switch u.Scheme {
	case schemeHttp:
		if err := config.LoadPasswordFile(&c.Password, c.PasswordFile); err != nil {
			return err
		}
		if c.Username == "" || c.Password == "" {
			return errors.New("http notifications source requires a username and password")
		}

	case schemeHttps:
		c.TlsOptions.Enable = true
		if c.TlsOptions.Cert == "" && c.Username == "" {
			return errors.New("https notifications source requires either certificates or username and password")
		}

		if (c.TlsOptions.Cert == "") != (c.TlsOptions.Key == "") {
			return errors.New("https notifications source requires either both cert and key or none")
		}

		if c.Username != "" {
			if err := config.LoadPasswordFile(&c.Password, c.PasswordFile); err != nil {
				return err
			}
			if c.Password == "" {
				return errors.New("https notifications source with a username require a password")
			}
		}

	case schemeUnix:
		if c.Username != "" || c.Password != "" || c.PasswordFile != "" {
			return errors.New("unix notifications source uses no username/password authentication")
		}

	default:
		return errors.Errorf("unsupported notifications scheme %q", u.Scheme)
	}

	return nil
}
