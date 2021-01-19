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
