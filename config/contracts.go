package config

// Validator is an interface that must be implemented by any configuration struct used in [FromYAMLFile].
//
// The Validate method checks the configuration values and
// returns an error if any value is invalid or missing when required.
//
// For fields such as file paths, the responsibility of Validate is limited to
// verifying the presence and format of the value,
// not checking external conditions like file existence or readability.
// This principle applies generally to any field where external validation
// (e.g., network availability, resource accessibility) is beyond the scope of basic configuration validation.
type Validator interface {
	// Validate checks the configuration values and
	// returns an error if any value is invalid or missing when required.
	Validate() error
}
