/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package transformer

import (
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/types"
)

// AddAttribute adds a cloudevents attribute (if missing) during the encoding process
func AddAttribute(attributeKind spec.Kind, value interface{}) binding.TransformerFunc {
	return SetAttribute(attributeKind, func(i2 interface{}) (i interface{}, err error) {
		if types.IsZero(i2) {
			return value, nil
		}
		return i2, nil
	})
}

// AddExtension adds a cloudevents extension (if missing) during the encoding process
func AddExtension(name string, value interface{}) binding.TransformerFunc {
	return SetExtension(name, func(i2 interface{}) (i interface{}, err error) {
		if types.IsZero(i2) {
			return value, nil
		}
		return i2, nil
	})
}
