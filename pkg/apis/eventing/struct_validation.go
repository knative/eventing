/*
Copyright 2020 The Knative Authors

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

package eventing

import (
	"reflect"
)

// HasMultipleSetFields checks if the struct has more than one field with a non-zero value
func HasMultipleSetFields(o interface{}) bool {
	val := reflect.ValueOf(o)
	fieldSet := false
	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		if !IsEmptyField(f) {
			if !fieldSet {
				fieldSet = true
			} else {
				return true
			}
		}
	}
	return false
}

// IsEmptyField checks if a struct field is empty by comparing it against its zero value
func IsEmptyField(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface, reflect.Chan:
		return v.IsNil()
	case reflect.Bool:
		return !v.Bool()
	}
	return false
}
