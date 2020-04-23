package transformer

import (
	"context"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/event"
)

// Add cloudevents attribute (if missing) during the encoding process
func AddAttribute(attributeKind spec.Kind, value interface{}) binding.TransformerFactory {
	return addAttributeTransformerFactory{attributeKind: attributeKind, value: value}
}

// Add cloudevents extension (if missing) during the encoding process
func AddExtension(name string, value interface{}) binding.TransformerFactory {
	return addExtensionTransformerFactory{name: name, value: value}
}

type addAttributeTransformerFactory struct {
	attributeKind spec.Kind
	value         interface{}
}

func (a addAttributeTransformerFactory) StructuredTransformer(binding.StructuredWriter) binding.StructuredWriter {
	return nil
}

func (a addAttributeTransformerFactory) BinaryTransformer(encoder binding.BinaryWriter) binding.BinaryWriter {
	return &addAttributeTransformer{
		BinaryWriter:  encoder,
		attributeKind: a.attributeKind,
		value:         a.value,
		found:         false,
	}
}

func (a addAttributeTransformerFactory) EventTransformer() binding.EventTransformer {
	return func(event *event.Event) error {
		v := spec.VS.Version(event.SpecVersion())
		if v == nil {
			return fmt.Errorf("spec version %s invalid", event.SpecVersion())
		}
		if v.AttributeFromKind(a.attributeKind).Get(event.Context) == nil {
			return v.AttributeFromKind(a.attributeKind).Set(event.Context, a.value)
		}
		return nil
	}
}

type addExtensionTransformerFactory struct {
	name  string
	value interface{}
}

func (a addExtensionTransformerFactory) StructuredTransformer(binding.StructuredWriter) binding.StructuredWriter {
	return nil
}

func (a addExtensionTransformerFactory) BinaryTransformer(encoder binding.BinaryWriter) binding.BinaryWriter {
	return &addExtensionTransformer{
		BinaryWriter: encoder,
		name:         a.name,
		value:        a.value,
		found:        false,
	}
}

func (a addExtensionTransformerFactory) EventTransformer() binding.EventTransformer {
	return func(event *event.Event) error {
		if _, ok := event.Extensions()[a.name]; !ok {
			return event.Context.SetExtension(a.name, a.value)
		}
		return nil
	}
}

type addAttributeTransformer struct {
	binding.BinaryWriter
	attributeKind spec.Kind
	value         interface{}
	version       spec.Version
	found         bool
}

func (b *addAttributeTransformer) SetAttribute(attribute spec.Attribute, value interface{}) error {
	if attribute.Kind() == b.attributeKind {
		b.found = true
	}
	b.version = attribute.Version()
	return b.BinaryWriter.SetAttribute(attribute, value)
}

func (b *addAttributeTransformer) End(ctx context.Context) error {
	if !b.found {
		err := b.BinaryWriter.SetAttribute(b.version.AttributeFromKind(b.attributeKind), b.value)
		if err != nil {
			return err
		}
	}
	return b.BinaryWriter.End(ctx)
}

type addExtensionTransformer struct {
	binding.BinaryWriter
	name  string
	value interface{}
	found bool
}

func (b *addExtensionTransformer) SetExtension(name string, value interface{}) error {
	if name == b.name {
		b.found = true
	}
	return b.BinaryWriter.SetExtension(name, value)
}

func (b *addExtensionTransformer) End(ctx context.Context) error {
	if !b.found {
		err := b.BinaryWriter.SetExtension(b.name, b.value)
		if err != nil {
			return err
		}
	}
	return b.BinaryWriter.End(ctx)
}
