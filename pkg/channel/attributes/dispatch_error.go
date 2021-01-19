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
	"context"
	"encoding/json"
	"errors"

	"github.com/cloudevents/sdk-go/v2/binding"
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

	// Truncate Data To Max Size If Too Large
	if len(data) > MaxDispatchErrorExtensionDataBytes {
		data = data[:MaxDispatchErrorExtensionDataBytes]
	}

	// Create & Return New DispatchErrorExtension Struct
	return DispatchErrorExtension{
		Code: code,
		Data: data,
	}
}

// Set the DispatchErrorExtension as an Extension Attribute on the specified Message
func SetDispatchErrorExtension(ctx context.Context, dispatchErrorExtension DispatchErrorExtension, message binding.Message) (binding.Message, error) {

	// Return Invalid Message Arguments
	if ctx == nil || message == nil {
		return message, errors.New("invalid arguments")
	}

	// Marshall Specified DispatchErrorExtension Into JSON Bytes
	dispatchErrorExtensionJsonBytes, err := json.Marshal(dispatchErrorExtension)
	if err != nil {
		return message, err
	}

	// Convert The Specified Binding Message To An Event
	event, err := binding.ToEvent(ctx, message)
	if err != nil {
		return message, err
	}

	// Set The DispatchErrorExtension On The Event
	event.SetExtension(DispatchErrorExtensionKey, dispatchErrorExtensionJsonBytes)

	// Convert The Event Back To A Binding Message & Return Success
	return binding.ToMessage(event), nil
}
