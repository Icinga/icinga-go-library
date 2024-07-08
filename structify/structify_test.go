package structify

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

type initerTest struct {
	S string `test:"s"`
	I int8   `test:"i"`
}

type stringTest struct {
	S string `test:"s"`
}

func testIniter(p any) {
	pIniterTest, ok := p.(*initerTest)
	if !ok {
		panic("p is not of type initerTest")
	}
	pIniterTest.I = 42
}

func TestMakeMapStructifier(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ms := MakeMapStructifier(reflect.TypeOf(struct{}{}), "", nil)
		require.NotNil(t, ms)

		actual, err := ms(nil)
		require.NoError(t, err)
		require.Equal(t, &struct{}{}, actual)
	})

	t.Run("unsupported", func(t *testing.T) {
		require.Panics(t, func() {
			MakeMapStructifier(reflect.TypeOf(struct {
				S struct{} `test:"s"`
			}{}), "test", nil)
		})
	})

	t.Run("embedded-ignored", func(t *testing.T) {
		require.NotPanics(t, func() {
			MakeMapStructifier(reflect.TypeOf(struct {
				Embedded struct {
					Ignored struct{} `test:"s"`
				}
			}{}), "test", nil)
		})
	})

	subtests := []struct {
		name   string
		initer func(any)
		input  map[string]any
		error  bool
		output any
	}{
		{
			name:   "initer_only",
			initer: testIniter,
			output: &initerTest{I: 42},
		},
		{
			name:   "initer_coexists",
			initer: testIniter,
			input:  map[string]any{"s": "foobar"},
			output: &initerTest{S: "foobar", I: 42},
		},
		{
			name:   "initer_overwritten",
			initer: testIniter,
			input:  map[string]any{"s": "foobar", "i": "23"},
			output: &initerTest{S: "foobar", I: 23},
		},
		{
			name:  "unexported",
			input: map[string]any{"s": "foobar"},
			output: &struct {
				s string `test:"s"`
			}{},
		},
		{
			name:  "no_tag",
			input: map[string]any{"s": "foobar"},
			output: &struct {
				S string
			}{},
		},
		{
			name:  "empty_tag",
			input: map[string]any{"s": "foobar"},
			output: &struct {
				S string `test:""`
			}{},
		},
		{
			name:  "dash_tag",
			input: map[string]any{"s": "foobar"},
			output: &struct {
				S string `test:"-"`
			}{},
		},
		{name: "missing_map", output: &stringTest{}},
		{
			name:  "not_string",
			input: map[string]any{"u": uint8(255)},
			output: &struct {
				U uint8 `test:"u"`
			}{},
		},
		{
			name:  "TextUnmarshaler",
			input: map[string]any{"boolean": "1"},
			output: &struct {
				Boolean types.Bool `test:"boolean"`
			}{types.Bool{Bool: true, Valid: true}},
		},
		{
			name:  "TextUnmarshaler_error",
			input: map[string]any{"boolean": "INVALID"},
			error: true,
			output: &struct {
				Boolean types.Bool `test:"boolean"`
			}{},
		},
		{
			name:   "string",
			input:  map[string]any{"s": "foobar"},
			output: &stringTest{S: "foobar"},
		},
		{
			name:  "pstring",
			input: map[string]any{"s": "foobar"},
			output: &struct {
				S *string `test:"s"`
			}{S: func(s string) *string { return &s }("foobar")},
		},
		{
			name:  "uint8",
			input: map[string]any{"u": "255"},
			output: &struct {
				U uint8 `test:"u"`
			}{U: 255},
		},
		{
			name:  "uint8_error",
			input: map[string]any{"u": "256"},
			error: true,
			output: &struct {
				U uint8 `test:"u"`
			}{},
		},
		{
			name:  "uint16",
			input: map[string]any{"u": "65535"},
			output: &struct {
				U uint16 `test:"u"`
			}{U: 65535},
		},
		{
			name:  "uint16_error",
			input: map[string]any{"u": "65536"},
			error: true,
			output: &struct {
				U uint16 `test:"u"`
			}{},
		},
		{
			name:  "uint32",
			input: map[string]any{"u": "4294967295"},
			output: &struct {
				U uint32 `test:"u"`
			}{U: 4294967295},
		},
		{
			name:  "uint32_error",
			input: map[string]any{"u": "4294967296"},
			error: true,
			output: &struct {
				U uint32 `test:"u"`
			}{},
		},
		{
			name:  "uint64",
			input: map[string]any{"u": "18446744073709551615"},
			output: &struct {
				U uint64 `test:"u"`
			}{U: 18446744073709551615},
		},
		{
			name:  "uint64_error",
			input: map[string]any{"u": "18446744073709551616"},
			error: true,
			output: &struct {
				U uint64 `test:"u"`
			}{},
		},
		{
			name:  "int8",
			input: map[string]any{"i": "-128"},
			output: &struct {
				I int8 `test:"i"`
			}{I: -128},
		},
		{
			name:  "int8_error",
			input: map[string]any{"i": "-129"},
			error: true,
			output: &struct {
				I int8 `test:"i"`
			}{},
		},
		{
			name:  "int16",
			input: map[string]any{"i": "-32768"},
			output: &struct {
				I int16 `test:"i"`
			}{I: -32768},
		},
		{
			name:  "int16_error",
			input: map[string]any{"i": "-32769"},
			error: true,
			output: &struct {
				I int16 `test:"i"`
			}{},
		},
		{
			name:  "int32",
			input: map[string]any{"i": "-2147483648"},
			output: &struct {
				I int32 `test:"i"`
			}{I: -2147483648},
		},
		{
			name:  "int32_error",
			input: map[string]any{"i": "-2147483649"},
			error: true,
			output: &struct {
				I int32 `test:"i"`
			}{},
		},
		{
			name:  "int64",
			input: map[string]any{"i": "-9223372036854775808"},
			output: &struct {
				I int64 `test:"i"`
			}{I: -9223372036854775808},
		},
		{
			name:  "int64_error",
			input: map[string]any{"i": "-9223372036854775809"},
			error: true,
			output: &struct {
				I int64 `test:"i"`
			}{},
		},
		{
			name:  "float32",
			input: map[string]any{"f": "3.4028235e+38"},
			output: &struct {
				F float32 `test:"f"`
			}{F: 3.4028235e+38},
		},
		{
			name:  "float32_error",
			input: map[string]any{"f": "3.4028236e+38"},
			error: true,
			output: &struct {
				F float32 `test:"f"`
			}{},
		},
		{
			name:  "float64",
			input: map[string]any{"f": "1.7976931348623157e+308"},
			output: &struct {
				F float64 `test:"f"`
			}{F: 1.7976931348623157e+308},
		},
		{
			name:  "float64_error",
			input: map[string]any{"f": "1.7976931348623158e+380"},
			error: true,
			output: &struct {
				F float64 `test:"f"`
			}{},
		},
		{
			name:  "inline",
			input: map[string]any{"s": "foobar"},
			output: &struct {
				Inline stringTest `test:",inline"`
			}{Inline: stringTest{S: "foobar"}},
		},
		{
			name:  "missing_inline",
			input: map[string]any{"s": "foobar"},
			output: &struct {
				Inline stringTest
			}{},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			ms := MakeMapStructifier(reflect.TypeOf(st.output).Elem(), "test", st.initer)
			require.NotNil(t, ms)

			if actual, err := ms(st.input); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			}
		})
	}
}
