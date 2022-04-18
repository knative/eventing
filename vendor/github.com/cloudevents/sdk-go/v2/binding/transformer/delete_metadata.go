/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package transformer

import (
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
)

// DeleteAttribute deletes a cloudevents attribute during the encoding process
func DeleteAttribute(attributeKind spec.Kind) binding.TransformerFunc {
	return SetAttribute(attributeKind, func(i2 interface{}) (i interface{}, err error) {
		return nil, nil
	})
}

// DeleteExtension deletes a cloudevents extension during the encoding process
func DeleteExtension(name string) binding.TransformerFunc {
	return SetExtension(name, func(i2 interface{}) (i interface{}, err error) {
		return nil, nil
	})
}
