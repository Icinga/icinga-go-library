package objectpacker

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"sort"
)

// PackAny packs any JSON-encodable value (ex. structs, also ignores interfaces like encoding.TextMarshaler)
// to a BSON-similar format suitable for consistent hashing. Spec:
// https://github.com/Icinga/icinga2/blob/2cb995e/lib/base/object-packer.cpp#L222-L231
func PackAny(in interface{}, out io.Writer) error {
	return packValue(reflect.ValueOf(in), out)
}

var tByte = reflect.TypeOf(byte(0))
var tBytes = reflect.TypeOf([]uint8(nil))

// packValue does the actual job of packAny and just exists for recursion w/o unneccessary reflect.ValueOf calls.
func packValue(in reflect.Value, out io.Writer) error {
	switch kind := in.Kind(); kind {
	case reflect.Invalid: // nil
		_, err := out.Write([]byte{0})
		return err
	case reflect.Bool:
		if in.Bool() {
			_, err := out.Write([]byte{2})
			return err
		} else {
			_, err := out.Write([]byte{1})
			return err
		}
	case reflect.Float64:
		if _, err := out.Write([]byte{3}); err != nil {
			return err
		}

		return binary.Write(out, binary.BigEndian, in.Float())
	case reflect.Array, reflect.Slice:
		if typ := in.Type(); typ.Elem() == tByte {
			if kind == reflect.Array {
				if !in.CanAddr() {
					vNewElem := reflect.New(typ).Elem()
					vNewElem.Set(in)
					in = vNewElem
				}

				in = in.Slice(0, in.Len())
			}

			// Pack []byte as string, not array of numbers.
			return packString(in.Convert(tBytes). // Support types.Binary
								Interface().([]uint8), out)
		}

		if _, err := out.Write([]byte{5}); err != nil {
			return err
		}

		l := in.Len()
		if err := binary.Write(out, binary.BigEndian, uint64(l)); err != nil {
			return err
		}

		for i := 0; i < l; i++ {
			if err := packValue(in.Index(i), out); err != nil {
				return err
			}
		}

		// If there aren't any values to pack, ...
		if l < 1 {
			// ... create one and pack it - panics on disallowed type.
			_ = packValue(reflect.Zero(in.Type().Elem()), ioutil.Discard)
		}

		return nil
	case reflect.Interface:
		return packValue(in.Elem(), out)
	case reflect.Map:
		type kv struct {
			key   []byte
			value reflect.Value
		}

		if _, err := out.Write([]byte{6}); err != nil {
			return err
		}

		l := in.Len()
		if err := binary.Write(out, binary.BigEndian, uint64(l)); err != nil {
			return err
		}

		sorted := make([]kv, 0, l)

		{
			iter := in.MapRange()
			for iter.Next() {
				// Not just stringify the key (below), but also pack it (here) - panics on disallowed type.
				_ = packValue(iter.Key(), ioutil.Discard)

				sorted = append(sorted, kv{[]byte(fmt.Sprint(iter.Key().Interface())), iter.Value()})
			}
		}

		sort.Slice(sorted, func(i, j int) bool { return bytes.Compare(sorted[i].key, sorted[j].key) < 0 })

		for _, kv := range sorted {
			if err := binary.Write(out, binary.BigEndian, uint64(len(kv.key))); err != nil {
				return err
			}

			if _, err := out.Write(kv.key); err != nil {
				return err
			}

			if err := packValue(kv.value, out); err != nil {
				return err
			}
		}

		// If there aren't any key-value pairs to pack, ...
		if l < 1 {
			typ := in.Type()

			// ... create one and pack it - panics on disallowed type.
			_ = packValue(reflect.Zero(typ.Key()), ioutil.Discard)
			_ = packValue(reflect.Zero(typ.Elem()), ioutil.Discard)
		}

		return nil
	case reflect.Ptr:
		if in.IsNil() {
			err := packValue(reflect.Value{}, out)

			// Create a fictive referenced value and pack it - panics on disallowed type.
			_ = packValue(reflect.Zero(in.Type().Elem()), ioutil.Discard)

			return err
		} else {
			return packValue(in.Elem(), out)
		}
	case reflect.String:
		return packString([]byte(in.String()), out)
	default:
		panic("bad type: " + in.Kind().String())
	}
}

// packString deduplicates string packing of multiple locations in packValue.
func packString(in []byte, out io.Writer) error {
	if _, err := out.Write([]byte{4}); err != nil {
		return err
	}

	if err := binary.Write(out, binary.BigEndian, uint64(len(in))); err != nil {
		return err
	}

	_, err := out.Write(in)
	return err
}
