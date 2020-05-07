package transformer

import (
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
)

// Version converts the event context version to the specified one.
func Version(newVersion spec.Version) binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		_, sv := reader.GetAttribute(spec.SpecVersion)
		if newVersion.String() == sv {
			return nil
		}

		for _, newAttr := range newVersion.Attributes() {
			oldAttr, val := reader.GetAttribute(newAttr.Kind())
			if oldAttr != nil && val != nil {
				// Erase old attr
				err := writer.SetAttribute(oldAttr, nil)
				if err != nil {
					return nil
				}
				if newAttr.Kind() == spec.SpecVersion {
					err = writer.SetAttribute(newAttr, newVersion.String())
					if err != nil {
						return nil
					}
				} else {
					err = writer.SetAttribute(newAttr, val)
					if err != nil {
						return nil
					}
				}
			}
		}
		return nil
	}
}
