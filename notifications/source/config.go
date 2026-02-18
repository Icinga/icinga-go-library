package source

// Config defines all configuration for the Icinga Notifications API Client.
type Config struct {
	// Url points to the Icinga Notifications API, e.g., http://localhost:5680
	Url string `yaml:"url" env:"URL"`

	// Username is the API user for the Icinga Notifications API.
	Username string `yaml:"username" env:"USERNAME"`

	// Password is the API user's password for the Icinga Notifications API.
	Password string `yaml:"password" env:"PASSWORD,unset"` // #nosec G117 -- exported password field
}
