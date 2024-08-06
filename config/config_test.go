package config

import (
	"errors"
	"github.com/stretchr/testify/require"
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
		name  string
		opts  EnvOptions
		io    Validator
		error bool
	}{
		{"nil", EnvOptions{}, nil, true},
		{"nonptr", EnvOptions{}, simpleValidator{}, true},
		{"nilptr", EnvOptions{}, (*simpleValidator)(nil), true},
		{"defaulterr", EnvOptions{}, new(nonStructValidator), true},
		{"parseeerr", EnvOptions{Environment: map[string]string{"FOO": "bar"}}, &simpleValidator{}, true},
		{"invalid", EnvOptions{Environment: map[string]string{"FOO": "23"}}, &simpleValidator{}, true},
		{"simple", EnvOptions{Environment: map[string]string{"FOO": "42"}}, &simpleValidator{42}, false},
		{"default", EnvOptions{}, &defaultValidator{42}, false},
		{"override", EnvOptions{Environment: map[string]string{"FOO": "23"}}, &defaultValidator{23}, false},
		{"prefix", EnvOptions{Environment: map[string]string{"PREFIX_FOO": "42"}, Prefix: "PREFIX_"}, &simpleValidator{42}, false},
		{"nested", EnvOptions{Environment: map[string]string{"PREFIX_FOO": "42"}}, &prefixValidator{simpleValidator{42}}, false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Validator
			if vActual := reflect.ValueOf(st.io); vActual != (reflect.Value{}) {
				if vActual.Kind() == reflect.Ptr && !vActual.IsNil() {
					vActual = reflect.New(vActual.Type().Elem())
				}

				actual = vActual.Interface().(Validator)
			}

			if err := FromEnv(actual, st.opts); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.io, actual)
			}
		})
	}
}
