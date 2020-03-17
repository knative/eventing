package transformer

import (
	"fmt"

	"github.com/cloudevents/sdk-go/pkg/binding"
	"github.com/cloudevents/sdk-go/pkg/binding/spec"
	"github.com/cloudevents/sdk-go/pkg/event"
)

// Delete cloudevents attribute during the encoding process
func DeleteAttribute(attributeKind spec.Kind) binding.TransformerFactory {
	return deleteAttributeTransformerFactory{attributeKind: attributeKind}
}

// Delete cloudevents extension during the encoding process
func DeleteExtension(name string) binding.TransformerFactory {
	return deleteExtensionTransformerFactory{name: name}
}

type deleteAttributeTransformerFactory struct {
	attributeKind spec.Kind
}

func (a deleteAttributeTransformerFactory) StructuredTransformer(binding.StructuredWriter) binding.StructuredWriter {
	return nil
}

func (a deleteAttributeTransformerFactory) BinaryTransformer(encoder binding.BinaryWriter) binding.BinaryWriter {
	return &deleteAttributeTransformer{
		BinaryWriter:  encoder,
		attributeKind: a.attributeKind,
	}
}

func (a deleteAttributeTransformerFactory) EventTransformer() binding.EventTransformer {
	return func(event *event.Event) error {
		v := spec.VS.Version(event.SpecVersion())
		if v == nil {
			return fmt.Errorf("spec version %s invalid", event.SpecVersion())
		}
		if v.AttributeFromKind(a.attributeKind).Get(event.Context) != nil {
			return v.AttributeFromKind(a.attributeKind).Delete(event.Context)
		}
		return nil
	}
}

type deleteExtensionTransformerFactory struct {
	name string
}

func (a deleteExtensionTransformerFactory) StructuredTransformer(binding.StructuredWriter) binding.StructuredWriter {
	return nil
}

func (a deleteExtensionTransformerFactory) BinaryTransformer(encoder binding.BinaryWriter) binding.BinaryWriter {
	return &deleteExtensionTransformer{
		BinaryWriter: encoder,
		name:         a.name,
	}
}

func (a deleteExtensionTransformerFactory) EventTransformer() binding.EventTransformer {
	return func(event *event.Event) error {
		return event.Context.SetExtension(a.name, nil)
	}
}

type deleteAttributeTransformer struct {
	binding.BinaryWriter
	attributeKind spec.Kind
}

func (b *deleteAttributeTransformer) SetAttribute(attribute spec.Attribute, value interface{}) error {
	if attribute.Kind() == b.attributeKind {
		return nil
	}
	return b.BinaryWriter.SetAttribute(attribute, value)
}

type deleteExtensionTransformer struct {
	binding.BinaryWriter
	name string
}

func (b *deleteExtensionTransformer) SetExtension(name string, value interface{}) error {
	if b.name == name {
		return nil
	}
	return b.BinaryWriter.SetExtension(name, value)
}
