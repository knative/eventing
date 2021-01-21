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

package attributes

import (
	"encoding/json"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
)

const (
	DispatchErrorExtensionKey          = "knativedispatcherr" // Max 20 characters from "a-z0-9" starting with "knative"
	MaxDispatchErrorExtensionDataBytes = 1024
)

// The DispatchError Extension Value Struct
type DispatchErrorExtension struct {
	Code int    `json:"code,omitempty"`
	Data []byte `json:"data,omitempty"`
}

// DispatchErrorExtension Constructor
func NewDispatchErrorExtension(code int, data []byte) DispatchErrorExtension {
	return DispatchErrorExtension{Code: code, Data: data}
}

// Returns An AddExtension Transformer For The DispatchErrorExtension Attribute
func (d *DispatchErrorExtension) AddExtensionTransformer() (binding.TransformerFunc, error) {

	// The DispatchErrorExtension To Add As An Extension
	dispatchErrorExtension := *d

	// Trim Data Field To Max Length If Necessary (Immutable Copy)
	if len(d.Data) > MaxDispatchErrorExtensionDataBytes {
		dispatchErrorExtension = NewDispatchErrorExtension(d.Code, d.Data[:MaxDispatchErrorExtensionDataBytes])
	}

	// Marshal The DispatchErrorExtension Into JSON Bytes
	jsonBytes, err := json.Marshal(dispatchErrorExtension)
	if err != nil {
		return nil, err
	}

	// Create & Return A CloudEvent AddExtension Transformer
	return transformer.AddExtension(DispatchErrorExtensionKey, jsonBytes), nil
}
