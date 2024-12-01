// Package config provides utilities for configuration parsing and loading.
// It includes functionality for handling command-line flags and loading configuration from YAML files,
// with additional support for setting default values and validation.
// Additionally, it provides a struct that defines common settings for a TLS client.
//
// Example usage:
//
//	type Config struct {
//		ServerAddress string     `yaml:"server_address" default:"localhost:8080"`
//		TLS           config.TLS `yaml:",inline"`
//	}
//
//	// Validate implements the Validator interface.
//	func (c *Config) Validate() error {
//		if _, _, err := net.SplitHostPort(c.ServerAddress); err != nil {
//			return errors.Wrapf(err, "invalid server address: %s", c.ServerAddress)
//		}
//
//		return nil
//	}
//
//	type Flags struct {
//		Config string `short:"c" long:"config" description:"Path to config file" required:"true"`
//	}
//
//	func main() {
//		var flags Flags
//		if err := config.ParseFlags(&flags); err != nil {
//			log.Fatalf("error parsing flags: %v", err)
//		}
//
//		var cfg Config
//		if err := config.FromYAMLFile(flags.Config, &cfg); err != nil {
//			log.Fatalf("error loading config: %v", err)
//		}
//
//		tlsCfg, err := cfg.TLS.MakeConfig("icinga.com")
//		if err != nil {
//			log.Fatalf("error creating TLS config: %v", err)
//		}
//
//		// ...
//	}
package config

import (
	stderrors "errors"
	"fmt"
	"github.com/caarlos0/env/v11"
	"github.com/creasty/defaults"
	"github.com/goccy/go-yaml"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"os"
	"reflect"
)

// ErrInvalidArgument is the error returned by [ParseFlags] or [FromYAMLFile] if
// its parsing result cannot be stored in the value pointed to by the designated passed argument which
// must be a non-nil struct pointer.
var ErrInvalidArgument = stderrors.New("invalid argument")

// ErrInvalidConfiguration is attached to errors returned by [FromYAMLFile] or [FromEnv] when
// the configuration is invalid,
// i.e. if the Validate method of the provided [Validator] interface returns an error,
// which is then propagated by these functions.
// Note that for such errors, errors.Is() will recognize both ErrInvalidConfiguration and
// the original errors returned from Validate.
var ErrInvalidConfiguration = stderrors.New("invalid configuration")

// FromYAMLFile parses the given YAML file and stores the result
// in the value pointed to by v. If v is nil or not a struct pointer,
// FromYAMLFile returns an [ErrInvalidArgument] error.
// It is possible to define default values via the struct tag `default`.
// The function also validates the configuration using the Validate method
// of the provided [Validator] interface.
// Any error returned from Validate is propagated with [ErrInvalidConfiguration] attached,
// allowing errors.Is() checks on the returned errors to recognize both ErrInvalidConfiguration and
// the original errors returned from Validate.
//
// Example usage:
//
//	type Config struct {
//		ServerAddress string `yaml:"server_address" default:"localhost:8080"`
//	}
//
//	// Validate implements the Validator interface.
//	func (c *Config) Validate() error {
//		if _, _, err := net.SplitHostPort(c.ServerAddress); err != nil {
//			return errors.Wrapf(err, "invalid server address: %s", c.ServerAddress)
//		}
//
//		return nil
//	}
//
//	func main() {
//		var cfg Config
//		if err := config.FromYAMLFile("config.yml", &cfg); err != nil {
//			log.Fatalf("error loading config: %v", err)
//		}
//
//		// ...
//	}
func FromYAMLFile(name string, v Validator) error {
	if err := validateNonNilStructPointer(v); err != nil {
		return errors.WithStack(err)
	}

	// #nosec G304 -- Accept user-controlled input for config file.
	f, err := os.Open(name)
	if err != nil {
		return errors.Wrap(err, "can't open YAML file "+name)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	if err := defaults.Set(v); err != nil {
		return errors.Wrap(err, "can't set config defaults")
	}

	d := yaml.NewDecoder(f, yaml.DisallowUnknownField())
	if err := d.Decode(v); err != nil {
		return errors.Wrap(err, "can't parse YAML file "+name)
	}

	if err := v.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfiguration, errors.WithStack(err))
	}

	return nil
}

// EnvOptions is a type alias for [env.Options], so that only this package needs to import [env].
type EnvOptions = env.Options

// FromEnv parses environment variables and stores the result in the value pointed to by v.
// If v is nil or not a struct pointer, FromEnv returns an [ErrInvalidArgument] error.
// Any error returned from Validate is propagated with [ErrInvalidConfiguration] attached,
// allowing errors.Is() checks on the returned errors to recognize both ErrInvalidConfiguration and
// the original errors returned from Validate.
func FromEnv(v Validator, options EnvOptions) error {
	if err := validateNonNilStructPointer(v); err != nil {
		return errors.WithStack(err)
	}

	if err := defaults.Set(v); err != nil {
		return errors.Wrap(err, "can't set config defaults")
	}

	if err := env.ParseWithOptions(v, options); err != nil {
		return errors.Wrap(err, "can't parse environment variables")
	}

	if err := v.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfiguration, errors.WithStack(err))
	}

	return nil
}

// ParseFlags parses CLI flags and stores the result
// in the value pointed to by v. If v is nil or not a struct pointer,
// ParseFlags returns an [ErrInvalidArgument] error.
// ParseFlags adds a default Help Options group,
// which contains the options -h and --help.
// If either option is specified on the command line,
// ParseFlags prints the help message to [os.Stdout] and exits.
// Note that errors are not printed automatically,
// so error handling is the sole responsibility of the caller.
//
// Example usage:
//
//	type Flags struct {
//		Config string `short:"c" long:"config" description:"Path to config file" required:"true"`
//	}
//
//	func main() {
//		var flags Flags
//		if err := config.ParseFlags(&flags); err != nil {
//			log.Fatalf("error parsing flags: %v", err)
//		}
//
//		// ...
//	}
func ParseFlags(v any) error {
	if err := validateNonNilStructPointer(v); err != nil {
		return errors.WithStack(err)
	}

	parser := flags.NewParser(v, flags.Default^flags.PrintErrors)

	if _, err := parser.Parse(); err != nil {
		var flagErr *flags.Error
		if errors.As(err, &flagErr) && errors.Is(flagErr.Type, flags.ErrHelp) {
			_, _ = fmt.Fprintln(os.Stdout, flagErr)
			os.Exit(0)
		}

		return errors.Wrap(err, "can't parse CLI flags")
	}

	return nil
}

// validateNonNilStructPointer checks if the provided value is a non-nil pointer to a struct.
// It returns an error if the value is not a pointer, is nil, or does not point to a struct.
func validateNonNilStructPointer(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return errors.Wrapf(ErrInvalidArgument, "non-nil struct pointer expected, got %T", v)
	}

	return nil
}
