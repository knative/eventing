package transformer

import (
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
)

// SetAttribute sets a cloudevents attribute using the provided function.
// updater gets a zero value as input if no previous value was found. To test a zero value, use types.IsZero().
// updater must return nil, nil if the user wants to remove the attribute
func SetAttribute(attribute spec.Kind, updater func(interface{}) (interface{}, error)) binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		attr, oldVal := reader.GetAttribute(attribute)
		if attr == nil {
			// The spec version of this message doesn't support this attribute, skip this
			return nil
		}
		newVal, err := updater(oldVal)
		if err != nil {
			return err
		}
		return writer.SetAttribute(attr, newVal)
	}
}

// SetExtension sets a cloudevents extension using the provided function.
// updater gets a zero value as input if no previous value was found. To test a zero value, use types.IsZero()
// updater must return nil, nil if the user wants to remove the extension
func SetExtension(name string, updater func(interface{}) (interface{}, error)) binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		oldVal := reader.GetExtension(name)
		newVal, err := updater(oldVal)
		if err != nil {
			return err
		}
		return writer.SetExtension(name, newVal)
	}
}
