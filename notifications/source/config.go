package source

// Config defines all configuration for the Icinga Notifications API Client.
type Config struct {
	// ApiBaseUrl points to the Icinga Notifications API, e.g., http://localhost:5680
	ApiBaseUrl string `yaml:"api-base-url" env:"API_BASE_URL"`

	// Username is the API user for the Icinga Notifications API.
	Username string `yaml:"username" env:"USERNAME"`

	// Password is the API user's password for the Icinga Notifications API.
	Password string `yaml:"password" env:"PASSWORD,unset"`
}
