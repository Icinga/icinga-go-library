package config

import (
	"errors"
	"github.com/stretchr/testify/require"
	"os"
	"reflect"
	"testing"
)

type simpleValidator struct {
	Foo int `env:"FOO"`
}

func (sv simpleValidator) Validate() error {
	if sv.Foo == 42 {
		return nil
	} else {
		return errors.New("invalid value")
	}
}

type nonStructValidator int

func (nonStructValidator) Validate() error {
	return nil
}

type defaultValidator struct {
	Foo int `env:"FOO" default:"42"`
}

func (defaultValidator) Validate() error {
	return nil
}

type prefixValidator struct {
	Nested simpleValidator `envPrefix:"PREFIX_"`
}

func (prefixValidator) Validate() error {
	return nil
}

func TestFromEnv(t *testing.T) {
	subtests := []struct {
		name    string
		env     map[string]string
		options EnvOptions
		io      Validator
		error   bool
	}{
		{"nil", nil, EnvOptions{}, nil, true},
		{"nonptr", nil, EnvOptions{}, simpleValidator{}, true},
		{"nilptr", nil, EnvOptions{}, (*simpleValidator)(nil), true},
		{"defaulterr", nil, EnvOptions{}, new(nonStructValidator), true},
		{"parseeerr", map[string]string{"FOO": "bar"}, EnvOptions{}, &simpleValidator{}, true},
		{"invalid", map[string]string{"FOO": "23"}, EnvOptions{}, &simpleValidator{}, true},
		{"simple", map[string]string{"FOO": "42"}, EnvOptions{}, &simpleValidator{42}, false},
		{"default", nil, EnvOptions{}, &defaultValidator{42}, false},
		{"override", map[string]string{"FOO": "23"}, EnvOptions{}, &defaultValidator{23}, false},
		{"prefix", map[string]string{"PREFIX_FOO": "42"}, EnvOptions{Prefix: "PREFIX_"}, &simpleValidator{42}, false},
		{"nested", map[string]string{"PREFIX_FOO": "42"}, EnvOptions{}, &prefixValidator{simpleValidator{42}}, false},
	}

	allEnv := make(map[string]struct{})
	for _, st := range subtests {
		for k := range st.env {
			allEnv[k] = struct{}{}
		}
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			for k := range allEnv {
				require.NoError(t, os.Unsetenv(k))
			}

			for k, v := range st.env {
				require.NoError(t, os.Setenv(k, v))
			}

			var actual Validator
			if vActual := reflect.ValueOf(st.io); vActual != (reflect.Value{}) {
				if vActual.Kind() == reflect.Ptr && !vActual.IsNil() {
					vActual = reflect.New(vActual.Type().Elem())
				}

				actual = vActual.Interface().(Validator)
			}

			if err := FromEnv(actual, st.options); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.io, actual)
			}
		})
	}
}
