package flatten

import (
	"fmt"
	"github.com/icinga/icinga-go-library/types"
	"strconv"
)

// Flatten creates flat, one-dimensional maps from arbitrarily nested values, e.g. JSON.
func Flatten(value any, prefix string) map[string]types.String {
	var flatten func(string, any)
	flattened := make(map[string]types.String)

	flatten = func(key string, value any) {
		switch value := value.(type) {
		case map[string]any:
			if len(value) == 0 {
				flattened[key] = types.String{}
				break
			}

			for k, v := range value {
				flatten(key+"."+k, v)
			}
		case []any:
			if len(value) == 0 {
				flattened[key] = types.String{}
				break
			}

			for i, v := range value {
				flatten(key+"["+strconv.Itoa(i)+"]", v)
			}
		case nil:
			flattened[key] = types.MakeString("null")
		case float64:
			flattened[key] = types.MakeString(strconv.FormatFloat(value, 'f', -1, 64))
		default:
			flattened[key] = types.MakeString(fmt.Sprintf("%v", value))
		}
	}

	flatten(prefix, value)

	return flattened
}
