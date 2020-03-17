package transformer

import (
	"github.com/cloudevents/sdk-go/pkg/binding"
	"github.com/cloudevents/sdk-go/pkg/binding/spec"
)

// Sets a cloudevents attribute (if missing) to defaultValue or update it with updater function
func SetAttribute(attribute spec.Kind, defaultValue interface{}, updater func(interface{}) (interface{}, error)) []binding.TransformerFactory {
	return []binding.TransformerFactory{
		UpdateAttribute(attribute, updater),
		AddAttribute(attribute, defaultValue),
	}
}

// Sets a cloudevents extension (if missing) to defaultValue or update it with updater function
func SetExtension(name string, defaultValue interface{}, updater func(interface{}) (interface{}, error)) []binding.TransformerFactory {
	return []binding.TransformerFactory{
		UpdateExtension(name, updater),
		AddExtension(name, defaultValue),
	}
}
