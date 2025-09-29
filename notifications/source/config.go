package source

// Config defines all the required configuration for the Icinga Notifications API client.
type Config struct {
	ApiBaseUrl string `yaml:"api-base-url" env:"API_BASE_URL"`
	Username   string `yaml:"username" env:"USERNAME"`
	Password   string `yaml:"password" env:"PASSWORD,unset"`
}
