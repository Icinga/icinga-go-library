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

	// Icingaweb2Url is the base URL of the Icinga Web 2 setup in front of this source,
	// e.g. "https://example.com/icingaweb2".
	//
	// A source resolves the object URL it transmits with each event against this base. Icinga Notifications
	// hands that URL to the notified contacts and cannot complete a relative reference by itself, so it has
	// to be absolute. If left empty, the source submits its events without an object URL.
	Icingaweb2Url string `yaml:"icingaweb2_url" env:"ICINGAWEB2_URL"`

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

	// Icingaweb2UrlParsed holds the parsed Icingaweb2Url after validation of the config.
	//
	// This field is not part of the YAML config and is only populated after successful validation.
	// The resulting URL always ends with a trailing slash, making it easier to resolve relative paths against it.
	Icingaweb2UrlParsed *url.URL
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

	if c.Icingaweb2Url != "" {
		icingaweb2Url, err := url.Parse(c.Icingaweb2Url)
		if err != nil {
			return errors.Wrap(err, "cannot parse icingaweb2_url")
		}

		if !icingaweb2Url.IsAbs() {
			return errors.Errorf("icingaweb2_url has to be an absolute URL, got %q", c.Icingaweb2Url)
		}

		icingaweb2Url.RawQuery = "" // Ignore query params if provided, as they are not relevant for object URLs.
		// Ensure the URL ends with a trailing slash for easier resolution of relative paths.
		c.Icingaweb2UrlParsed = icingaweb2Url.JoinPath("/")
	}

	return nil
}
