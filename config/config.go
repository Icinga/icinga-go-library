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

// FromYAMLFile parses the given YAML file and stores the result
// in the value pointed to by v. If v is nil or not a struct pointer,
// FromYAMLFile returns an [ErrInvalidArgument] error.
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
		return errors.Wrap(err, "invalid configuration")
	}

	return nil
}

// EnvOptions is a type alias for [env.Options], so that only this package needs to import [env].
type EnvOptions = env.Options

// FromEnv parses environment variables and stores the result in the value pointed to by v.
// If v is nil or not a struct pointer, FromEnv returns an [ErrInvalidArgument] error.
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
		return errors.Wrap(err, "invalid configuration")
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
