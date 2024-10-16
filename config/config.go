package config

import (
	stderrors "errors"
	"fmt"
	"github.com/creasty/defaults"
	"github.com/goccy/go-yaml"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"os"
)

// ErrInvalidArgument is the error returned by [ParseFlags] or [FromYAMLFile] if
// its parsing result cannot be stored in the value pointed to by the designated passed argument which
// must be a non-nil pointer.
var ErrInvalidArgument = stderrors.New("invalid argument")

// FromYAMLFile parses the given YAML file and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// FromYAMLFile returns an [ErrInvalidArgument] error.
func FromYAMLFile[T any, V validatorPtr[T]](name string, v V) error {
	if v == nil {
		return errors.Wrap(ErrInvalidArgument, "got nil pointer")
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

// ParseFlags parses CLI flags and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// ParseFlags returns an [ErrInvalidArgument] error.
// ParseFlags adds a default Help Options group,
// which contains the options -h and --help.
// If either option is specified on the command line,
// ParseFlags prints the help message to [os.Stdout] and exits.
// Note that errors are not printed automatically,
// so error handling is the sole responsibility of the caller.
func ParseFlags[T any](v *T) error {
	if v == nil {
		return errors.Wrap(ErrInvalidArgument, "got nil pointer")
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
