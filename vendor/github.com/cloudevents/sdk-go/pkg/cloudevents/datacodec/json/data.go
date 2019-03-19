package json

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

func Decode(in, out interface{}) error {
	if in == nil {
		return nil
	}
	if out == nil {
		return fmt.Errorf("out is nil")
	}

	b, ok := in.([]byte) // TODO: I think there is fancy marshaling happening here. Fix with reflection?
	if !ok {
		var err error
		b, err = json.Marshal(in)
		if err != nil {
			return fmt.Errorf("[json] failed to marshal in: %s", err.Error())
		}
	}

	// TODO: the spec says json could be just data... At the moment we expect wrapped.
	if len(b) > 1 && (b[0] == byte('"') || (b[0] == byte('\\') && b[1] == byte('"'))) {
		s, err := strconv.Unquote(string(b))
		if err != nil {
			return fmt.Errorf("[json] failed to unquote in: %s", err.Error())
		}
		if len(s) > 0 && (s[0] == '{' || s[0] == '[') {
			// looks like json, use it
			b = []byte(s)
		}
	}

	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("[json] found bytes \"%s\", but failed to unmarshal: %s", string(b), err.Error())
	}
	return nil
}

func Encode(in interface{}) ([]byte, error) {
	if in == nil {
		return nil, nil
	}

	it := reflect.TypeOf(in)
	switch it.Kind() {
	case reflect.Slice:
		if it.Elem().Kind() == reflect.Uint8 {

			if b, ok := in.([]byte); ok && len(b) > 0 {
				// check to see if it is a pre-encoded byte string.
				if b[0] == byte('"') || b[0] == byte('{') || b[0] == byte('[') {
					return b, nil
				}
			}

		}
	}

	return json.Marshal(in)
}
