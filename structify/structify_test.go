package structify

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

type initerTest = struct {
	S string `test:"s"`
	I int8   `test:"i"`
}

type stringTest = struct {
	S string `test:"s"`
}

func testIniter(p any) {
	p.(*initerTest).I = 42
}

func TestMakeMapStructifier(t *testing.T) {
	subtests := []struct {
		name   string
		tag    string
		initer func(any)
		input  map[string]any
		error  bool
		output any
	}{
		{"empty", "", nil, nil, false, &struct{}{}},
		{"initer_only", "test", testIniter, nil, false, &initerTest{I: 42}},
		{"initer_coexists", "test", testIniter, map[string]any{"s": "foobar"}, false, &initerTest{S: "foobar", I: 42}},
		{"initer_overwritten", "test", testIniter, map[string]any{
			"s": "foobar",
			"i": "23",
		}, false, &initerTest{S: "foobar", I: 23}},
		{"unexported", "test", nil, map[string]any{"s": "foobar"}, false, &struct {
			s string `test:"s"`
		}{}},
		{"no_tag", "test", nil, map[string]any{"s": "foobar"}, false, &struct {
			S string
		}{}},
		{"empty_tag", "test", nil, map[string]any{"s": "foobar"}, false, &struct {
			S string `test:""`
		}{}},
		{"dash_tag", "test", nil, map[string]any{"s": "foobar"}, false, &struct {
			S string `test:"-"`
		}{}},
		{"missing", "test", nil, nil, false, &stringTest{}},
		{"not_string", "test", nil, map[string]any{"u": uint8(255)}, false, &struct {
			U uint8 `test:"u"`
		}{}},
		{"TextUnmarshaler", "test", nil, map[string]any{"boolean": "1"}, false, &struct {
			Boolean types.Bool `test:"boolean"`
		}{types.Bool{Bool: true, Valid: true}}},
		{"TextUnmarshaler_error", "test", nil, map[string]any{"boolean": "INVALID"}, true, &struct {
			Boolean types.Bool `test:"boolean"`
		}{}},
		{"string", "test", nil, map[string]any{"s": "foobar"}, false, &stringTest{S: "foobar"}},
		{"pstring", "test", nil, map[string]any{"s": "foobar"}, false, &struct {
			S *string `test:"s"`
		}{S: func(s string) *string { return &s }("foobar")}},
		{"uint8", "test", nil, map[string]any{"u": "255"}, false, &struct {
			U uint8 `test:"u"`
		}{U: 255}},
		{"uint8_error", "test", nil, map[string]any{"u": "256"}, true, &struct {
			U uint8 `test:"u"`
		}{}},
		{"uint16", "test", nil, map[string]any{"u": "65535"}, false, &struct {
			U uint16 `test:"u"`
		}{U: 65535}},
		{"uint16_error", "test", nil, map[string]any{"u": "65536"}, true, &struct {
			U uint16 `test:"u"`
		}{}},
		{"uint32", "test", nil, map[string]any{"u": "4294967295"}, false, &struct {
			U uint32 `test:"u"`
		}{U: 4294967295}},
		{"uint32_error", "test", nil, map[string]any{"u": "4294967296"}, true, &struct {
			U uint32 `test:"u"`
		}{}},
		{"uint64", "test", nil, map[string]any{"u": "18446744073709551615"}, false, &struct {
			U uint64 `test:"u"`
		}{U: 18446744073709551615}},
		{"uint64_error", "test", nil, map[string]any{"u": "18446744073709551616"}, true, &struct {
			U uint64 `test:"u"`
		}{}},
		{"int8", "test", nil, map[string]any{"i": "-128"}, false, &struct {
			I int8 `test:"i"`
		}{I: -128}},
		{"int8_error", "test", nil, map[string]any{"i": "-129"}, true, &struct {
			I int8 `test:"i"`
		}{}},
		{"int16", "test", nil, map[string]any{"i": "-32768"}, false, &struct {
			I int16 `test:"i"`
		}{I: -32768}},
		{"int16_error", "test", nil, map[string]any{"i": "-32769"}, true, &struct {
			I int16 `test:"i"`
		}{}},
		{"int32", "test", nil, map[string]any{"i": "-2147483648"}, false, &struct {
			I int32 `test:"i"`
		}{I: -2147483648}},
		{"int32_error", "test", nil, map[string]any{"i": "-2147483649"}, true, &struct {
			I int32 `test:"i"`
		}{}},
		{"int64", "test", nil, map[string]any{"i": "-9223372036854775808"}, false, &struct {
			I int64 `test:"i"`
		}{I: -9223372036854775808}},
		{"int64_error", "test", nil, map[string]any{"i": "-9223372036854775809"}, true, &struct {
			I int64 `test:"i"`
		}{}},
		{"float32", "test", nil, map[string]any{"f": "3.4028235e+38"}, false, &struct {
			F float32 `test:"f"`
		}{F: 3.4028235e+38}},
		{"float32_error", "test", nil, map[string]any{"f": "3.4028236e+38"}, true, &struct {
			F float32 `test:"f"`
		}{}},
		{"float64", "test", nil, map[string]any{"f": "1.7976931348623157e+308"}, false, &struct {
			F float64 `test:"f"`
		}{F: 1.7976931348623157e+308}},
		{"float64_error", "test", nil, map[string]any{"f": "1.7976931348623158e+380"}, true, &struct {
			F float64 `test:"f"`
		}{}},
		{"inline", "test", nil, map[string]any{"s": "foobar"}, false, &struct {
			Inline stringTest `test:",inline"`
		}{Inline: stringTest{S: "foobar"}}},
		{"missing_inline", "test", nil, map[string]any{"s": "foobar"}, false, &struct {
			Inline stringTest
		}{}},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			ms := MakeMapStructifier(reflect.TypeOf(st.output).Elem(), st.tag, st.initer)
			require.NotNil(t, ms)

			if actual, err := ms(st.input); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			}
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		require.Panics(t, func() {
			MakeMapStructifier(reflect.TypeOf(struct {
				S struct{} `test:"s"`
			}{}), "test", nil)
		})
	})
}
