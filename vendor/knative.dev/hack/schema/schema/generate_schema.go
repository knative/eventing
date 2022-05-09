/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	"knative.dev/hack/schema/docs"
)

var (
	trueVal = true

	zero      = 0.0
	int8Max   = float64(math.MaxInt8)
	int16Max  = float64(math.MaxInt16)
	uint8Max  = float64(math.MaxUint8)
	uint16Max = float64(math.MaxUint16)
	uint32Max = float64(math.MaxUint32)
)

func GenerateForType(t reflect.Type) JSONSchemaProps {
	s := generateStructSchema(t, true)
	return s
}

func generateSchema(t reflect.Type, history ...reflect.Type) JSONSchemaProps {
	if visited(t, history) {
		return JSONSchemaProps{
			Type: "object",
		}
	}

	switch k := t.Kind(); k {
	case reflect.Bool:
		return JSONSchemaProps{
			Type: "boolean",
		}
	case reflect.Int:
		return JSONSchemaProps{
			Type:   "integer",
			Format: "int32",
		}
	case reflect.Int8:
		return JSONSchemaProps{
			Type:    "integer",
			Maximum: &int8Max,
		}
	case reflect.Int16:
		return JSONSchemaProps{
			Type:    "integer",
			Maximum: &int16Max,
		}
	case reflect.Int32:
		return JSONSchemaProps{
			Type:   "integer",
			Format: "int32",
		}
	case reflect.Int64:
		return JSONSchemaProps{
			Type:   "integer",
			Format: "int64",
		}
	case reflect.Uint:
		return JSONSchemaProps{
			Type:    "integer",
			Minimum: &zero,
		}
	case reflect.Uint8:
		return JSONSchemaProps{
			Type:    "integer",
			Minimum: &zero,
			Maximum: &uint8Max,
		}
	case reflect.Uint16:
		return JSONSchemaProps{
			Type:    "integer",
			Minimum: &zero,
			Maximum: &uint16Max,
		}
	case reflect.Uint32:
		return JSONSchemaProps{
			Type:    "integer",
			Format:  "int64",
			Minimum: &zero,
			Maximum: &uint32Max,
		}
	case reflect.Uint64:
		return JSONSchemaProps{
			Type:    "integer",
			Format:  "int64",
			Minimum: &zero,
		}

	case reflect.Float32:
		return JSONSchemaProps{
			Type:   "number",
			Format: "float",
		}
	case reflect.Float64:
		return JSONSchemaProps{
			Type:   "number",
			Format: "double",
		}
	case reflect.Map:
		s := generateMapSchema(t)
		return s
	case reflect.Ptr:
		return generateSchema(t.Elem(), history...)
	case reflect.Slice:
		// From: https://pkg.go.dev/encoding/json#Marshal
		// Array and slice values encode as JSON arrays, except that []byte
		// encodes as a base64-encoded string, and a nil slice encodes as the
		// null JSON value.
		if t.Elem().Kind() == reflect.Uint8 {
			return JSONSchemaProps{
				Type: "string",
			}
		}
		s := generateSliceSchema(t)
		return s
	case reflect.String:
		return JSONSchemaProps{
			Type: "string",
		}
	case reflect.Struct:
		history = append(history, t)
		s := generateStructSchema(t, false, history...)
		return s
	case reflect.Interface:
		return JSONSchemaProps{
			Type: "object",
		}
	case reflect.Uintptr:
		fallthrough
	case reflect.Complex64:
		fallthrough
	case reflect.Complex128:
		fallthrough
	case reflect.Array:
		fallthrough
	case reflect.Chan:
		fallthrough
	case reflect.Func:
		fallthrough
	case reflect.UnsafePointer:
		panic(fmt.Errorf("can't handle kind %+v", t))
	default:
		panic(fmt.Errorf("unknown kind: %+v", t))
	}
}

var topLevelFieldsToSkip = map[string]struct{}{
	"TypeMeta":   {},
	"ObjectMeta": {},
}

func generateStructSchema(t reflect.Type, skipTopLevelCommon bool, history ...reflect.Type) JSONSchemaProps {
	s := JSONSchemaProps{
		Type:       "object",
		Properties: map[string]JSONSchemaProps{},
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if _, skip := topLevelFieldsToSkip[f.Name]; skip && skipTopLevelCommon {
			continue
		}
		name := f.Name
		jsonKey, present := f.Tag.Lookup("json")
		if present {
			split := strings.Split(jsonKey, ",")
			if split[0] != "" {
				name = split[0]
			}
		}

		fs := generateSchema(f.Type, history...)

		if selfJSONMarshaler(f.Type) {
			// The field marshals itself. Let's pretend it is a string.
			fs = JSONSchemaProps{
				Type: "string",
			}
		}

		if f.Anonymous {
			for n, p := range fs.Properties {
				s.Properties[n] = p
			}
			s.Required = append(s.Required, fs.Required...)
		} else {
			// Add docs
			doc, docSaysRequired, err := docs.GetDocsForField(t, f.Name)
			if err != nil {
				doc = fmt.Sprintf("not found: %v", err)
			}
			fs.Description = doc
			s.Properties[name] = fs
			switch docSaysRequired {
			case docs.Optional:
				// Nothing!
			case docs.Required:
				s.Required = append(s.Required, name)
			case docs.Unknown:
				// Doc didn't say anything, check if the type is a pointer.
				if f.Type.Kind() == reflect.Ptr {
					s.Required = append(s.Required, name)
				}
			}
		}

	}
	return s
}

// This should prevent loops for struct types.
func visited(t reflect.Type, history []reflect.Type) bool {
	for _, h := range history {
		// Look for a t in history.
		if t.String() == h.String() {
			return true
		}
	}
	return false
}

func generateMapSchema(t reflect.Type) JSONSchemaProps {
	if t.Key().Kind() != reflect.String {
		panic(fmt.Errorf("can't handle a non-string key: %+v", t))
	}
	return JSONSchemaProps{
		Type:                   "object",
		XPreserveUnknownFields: &trueVal,
	}
}

func generateSliceSchema(t reflect.Type, history ...reflect.Type) JSONSchemaProps {
	s := JSONSchemaProps{
		Type: "array",
	}
	// Special case bad actors.
	switch t.Elem().Name() {
	case "JSONSchemaProps":
		s.Items = &JSONSchemaPropsOrArray{
			Schema: &JSONSchemaProps{
				Type: "object",
			},
		}
	default:
		is := generateSchema(t.Elem(), history...)
		s.Items = &JSONSchemaPropsOrArray{
			Schema: &is,
		}
	}
	return s
}

// selfJSONMarshaler attempts to check if the field is marshals itself to and from JSON.
func selfJSONMarshaler(t reflect.Type) bool {
	jm := reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	ju := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	if !t.Implements(jm) && !reflect.PtrTo(t).Implements(jm) {
		return false
	}
	return t.Implements(ju) || reflect.PtrTo(t).Implements(ju)
}
