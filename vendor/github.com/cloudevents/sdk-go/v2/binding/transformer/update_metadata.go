package transformer

import (
	"fmt"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/event"
)

// Update cloudevents attribute (if present) using the provided function
func UpdateAttribute(attributeKind spec.Kind, updater func(interface{}) (interface{}, error)) binding.TransformerFactory {
	return updateAttributeTransformerFactory{attributeKind: attributeKind, updater: updater}
}

// Update cloudevents extension (if present) using the provided function
func UpdateExtension(name string, updater func(interface{}) (interface{}, error)) binding.TransformerFactory {
	return updateExtensionTransformerFactory{name: name, updater: updater}
}

type updateAttributeTransformerFactory struct {
	attributeKind spec.Kind
	updater       func(interface{}) (interface{}, error)
}

func (a updateAttributeTransformerFactory) StructuredTransformer(binding.StructuredWriter) binding.StructuredWriter {
	return nil
}

func (a updateAttributeTransformerFactory) BinaryTransformer(encoder binding.BinaryWriter) binding.BinaryWriter {
	return &updateAttributeTransformer{
		BinaryWriter:  encoder,
		attributeKind: a.attributeKind,
		updater:       a.updater,
	}
}

func (a updateAttributeTransformerFactory) EventTransformer() binding.EventTransformer {
	return func(event *event.Event) error {
		v := spec.VS.Version(event.SpecVersion())
		if v == nil {
			return fmt.Errorf("spec version %s invalid", event.SpecVersion())
		}
		if val := v.AttributeFromKind(a.attributeKind).Get(event.Context); val != nil {
			newVal, err := a.updater(val)
			if err != nil {
				return err
			}
			if newVal == nil {
				return v.AttributeFromKind(a.attributeKind).Delete(event.Context)
			} else {
				return v.AttributeFromKind(a.attributeKind).Set(event.Context, newVal)
			}
		}
		return nil
	}
}

type updateExtensionTransformerFactory struct {
	name    string
	updater func(interface{}) (interface{}, error)
}

func (a updateExtensionTransformerFactory) StructuredTransformer(binding.StructuredWriter) binding.StructuredWriter {
	return nil
}

func (a updateExtensionTransformerFactory) BinaryTransformer(encoder binding.BinaryWriter) binding.BinaryWriter {
	return &updateExtensionTransformer{
		BinaryWriter: encoder,
		name:         a.name,
		updater:      a.updater,
	}
}

func (a updateExtensionTransformerFactory) EventTransformer() binding.EventTransformer {
	return func(event *event.Event) error {
		if val, ok := event.Extensions()[a.name]; ok {
			newVal, err := a.updater(val)
			if err != nil {
				return err
			}
			return event.Context.SetExtension(a.name, newVal)
		}
		return nil
	}
}

type updateAttributeTransformer struct {
	binding.BinaryWriter
	attributeKind spec.Kind
	updater       func(interface{}) (interface{}, error)
}

func (b *updateAttributeTransformer) SetAttribute(attribute spec.Attribute, value interface{}) error {
	if attribute.Kind() == b.attributeKind {
		newVal, err := b.updater(value)
		if err != nil {
			return err
		}
		if newVal != nil {
			return b.BinaryWriter.SetAttribute(attribute, newVal)
		}
		return nil
	}
	return b.BinaryWriter.SetAttribute(attribute, value)
}

type updateExtensionTransformer struct {
	binding.BinaryWriter
	name    string
	updater func(interface{}) (interface{}, error)
}

func (b *updateExtensionTransformer) SetExtension(name string, value interface{}) error {
	if name == b.name {
		newVal, err := b.updater(value)
		if err != nil {
			return err
		}
		if newVal != nil {
			return b.BinaryWriter.SetExtension(name, newVal)
		}
		return nil
	}
	return b.BinaryWriter.SetExtension(name, value)
}
