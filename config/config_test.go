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
		{name: "nil", error: true},
		{name: "nonptr", io: simpleValidator{}, error: true},
		{name: "nilptr", io: (*simpleValidator)(nil), error: true},
		{name: "defaulterr", io: new(nonStructValidator), error: true},
		{
			name:  "parseeerr",
			opts:  EnvOptions{Environment: map[string]string{"FOO": "bar"}},
			io:    &simpleValidator{},
			error: true,
		},
		{
			name:  "invalid",
			opts:  EnvOptions{Environment: map[string]string{"FOO": "23"}},
			io:    &simpleValidator{},
			error: true,
		},
		{name: "simple", opts: EnvOptions{Environment: map[string]string{"FOO": "42"}}, io: &simpleValidator{42}},
		{name: "default", io: &defaultValidator{42}},
		{name: "override", opts: EnvOptions{Environment: map[string]string{"FOO": "23"}}, io: &defaultValidator{23}},
		{
			name: "prefix",
			opts: EnvOptions{Environment: map[string]string{"PREFIX_FOO": "42"}, Prefix: "PREFIX_"},
			io:   &simpleValidator{42},
		},
		{
			name: "nested",
			opts: EnvOptions{Environment: map[string]string{"PREFIX_FOO": "42"}},
			io:   &prefixValidator{simpleValidator{42}},
		},
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
