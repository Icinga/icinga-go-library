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
// must be a non-nil pointer.
var ErrInvalidArgument = stderrors.New("invalid argument")

// FromYAMLFile parses the given YAML file and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// FromYAMLFile returns an [ErrInvalidArgument] error.
// It is possible to define default values via the struct tag `default`.
// The function also validates the configuration using the Validate method
// of the provided [Validator] interface.
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
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.Wrapf(ErrInvalidArgument, "non-nil pointer expected, got %T", v)
	}

	// #nosec G304 -- Potential file inclusion via variable - Its purpose is to load any file name that is passed to it, so doesn't need to validate anything.
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
		return errors.Wrap(err, "invalid configuration")
	}

	return nil
}

// EnvOptions is a type alias for [env.Options], so that only this package needs to import [env].
type EnvOptions = env.Options

// FromEnv parses environment variables and stores the result in the value pointed to by v.
// If v is nil or not a pointer, FromEnv returns an [ErrInvalidArgument] error.
func FromEnv(v Validator, options EnvOptions) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.Wrapf(ErrInvalidArgument, "non-nil pointer expected, got %T", v)
	}

	if err := defaults.Set(v); err != nil {
		return errors.Wrap(err, "can't set config defaults")
	}

	if err := env.ParseWithOptions(v, options); err != nil {
		return errors.Wrap(err, "can't parse environment variables")
	}

	if err := v.Validate(); err != nil {
		return errors.Wrap(err, "invalid configuration")
	}

	return nil
}

// ParseFlags parses CLI flags and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
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
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.Wrapf(ErrInvalidArgument, "non-nil pointer expected, got %T", v)
	}

	parser := flags.NewParser(v, flags.Default^flags.PrintErrors)

	if _, err := parser.Parse(); err != nil {
		var flagErr *flags.Error
		if errors.As(err, &flagErr) && flagErr.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stdout, flagErr)
			os.Exit(0)
		}

		return errors.Wrap(err, "can't parse CLI flags")
	}

	return nil
}
