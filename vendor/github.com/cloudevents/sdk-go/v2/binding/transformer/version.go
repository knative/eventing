package transformer

import (
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/event"
)

// Converts the event context version to the specified one.
func Version(version spec.Version) binding.TransformerFactory {
	return versionTransformerFactory{version: version}
}

type versionTransformerFactory struct {
	version spec.Version
}

func (v versionTransformerFactory) StructuredTransformer(binding.StructuredWriter) binding.StructuredWriter {
	return nil // Not supported, must fallback to EventTransformer!
}

func (v versionTransformerFactory) BinaryTransformer(encoder binding.BinaryWriter) binding.BinaryWriter {
	return binaryVersionTransformer{BinaryWriter: encoder, version: v.version}
}

func (v versionTransformerFactory) EventTransformer() binding.EventTransformer {
	return func(e *event.Event) error {
		e.Context = v.version.Convert(e.Context)
		return nil
	}
}

type binaryVersionTransformer struct {
	binding.BinaryWriter
	version spec.Version
}

func (b binaryVersionTransformer) SetAttribute(attribute spec.Attribute, value interface{}) error {
	if attribute.Kind() == spec.SpecVersion {
		return b.BinaryWriter.SetAttribute(b.version.AttributeFromKind(spec.SpecVersion), b.version.String())
	}
	attributeInDifferentVersion := b.version.AttributeFromKind(attribute.Kind())
	return b.BinaryWriter.SetAttribute(attributeInDifferentVersion, value)
}
