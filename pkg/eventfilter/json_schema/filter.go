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

package json_schema

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/types"
	"github.com/qri-io/jsonschema"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/eventfilter"
)

type jsonSchemaFilter jsonschema.Schema

func NewJsonSchemaFilter(jsonSchema map[string]interface{}) (eventfilter.Filter, error) {
	schemaBytes, err := json.Marshal(jsonSchema)
	if err != nil {
		return nil, fmt.Errorf("error while marshalling the schema: %w", err)
	}

	rs := &jsonschema.Schema{}
	if err := json.Unmarshal(schemaBytes, rs); err != nil {
		return nil, fmt.Errorf("error while parsing the schema: %w", err)
	}

	return (*jsonSchemaFilter)(rs), nil
}

func (j *jsonSchemaFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	// The nice thing here would be to have an abstraction that makes event looks like a json
	// without triggering this copy.
	// Unfortunately this library doesn't allow it, hence we need to copy stuff inside a map to
	// give to the json schema validator
	json := make(map[string]interface{}, 4) // A CloudEvent has at least 4 attributes

	// We're leveraging the binding to efficiently copy-paste attributes and extensions into the json
	// to give to the json schema library.
	// It also guarantees no nil attributes (which might break the semantics of the schema, cloudevents spec never allows null values)
	err := binding.ToMessage(&event).ReadBinary(ctx, attributesWriter(json))
	if err != nil {
		// This should never happen in theory
		logging.FromContext(ctx).Warn("Error while trying to convert the input event attributes and extensions to json: ", err)
		return eventfilter.FailFilter
	}

	result := (*jsonschema.Schema)(j).Validate(ctx, json)
	if result.IsValid() {
		return eventfilter.PassFilter
	}
	return eventfilter.FailFilter
}

var _ eventfilter.Filter = (*jsonSchemaFilter)(nil)

type attributesWriter map[string]interface{}

func (a attributesWriter) SetAttribute(attribute spec.Attribute, value interface{}) error {
	v, err := coherceTypes(value)
	if err != nil {
		return err
	}
	a[attribute.Name()] = v
	return nil
}

func (a attributesWriter) SetExtension(name string, value interface{}) error {
	v, err := coherceTypes(value)
	if err != nil {
		return err
	}
	a[name] = v
	return nil
}

func (a attributesWriter) Start(ctx context.Context) error {
	return nil
}

func (a attributesWriter) SetData(data io.Reader) error {
	return nil
}

func (a attributesWriter) End(ctx context.Context) error {
	return nil
}

// Only some types has to be converted to strings
// The coherced types maps more or less to the official Cloudevents json schema
// https://github.com/cloudevents/spec/blob/master/spec.json
func coherceTypes(value interface{}) (interface{}, error) {
	validatedValue, err := types.Validate(value)
	if err != nil {
		// This should never happen because an event cannot contain an invalid type!
		return nil, err
	}

	switch t := validatedValue.(type) {
	case types.URI: // Use string form of URLs.
		return t.String(), nil
	case types.URIRef: // Use string form of URLs.
		return t.String(), nil
	case types.Timestamp: // Use string form of URLs.
		return t.String(), nil
	default:
		return validatedValue, nil
	}
}
