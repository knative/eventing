/*
Copyright 2021 The Knative Authors

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

package schema

// JSONSchemaProps is a JSON-Schema following Specification Draft 4 (http://json-schema.org/).
type JSONSchemaProps struct {
	Description          string                     `yaml:"description,omitempty"`
	Type                 string                     `yaml:"type,omitempty"`
	Format               string                     `yaml:"format,omitempty"`
	Default              *JSON                      `yaml:",omitempty"`
	Maximum              *float64                   `yaml:",omitempty"`
	ExclusiveMaximum     bool                       `yaml:",omitempty"`
	Minimum              *float64                   `yaml:",omitempty"`
	ExclusiveMinimum     bool                       `yaml:",omitempty"`
	MaxLength            *int64                     `yaml:",omitempty"`
	MinLength            *int64                     `yaml:",omitempty"`
	Pattern              string                     `yaml:",omitempty"`
	MaxItems             *int64                     `yaml:",omitempty"`
	MinItems             *int64                     `yaml:",omitempty"`
	UniqueItems          bool                       `yaml:",omitempty"`
	MultipleOf           *float64                   `yaml:",omitempty"`
	Enum                 []JSON                     `yaml:",omitempty"`
	MaxProperties        *int64                     `yaml:",omitempty"`
	MinProperties        *int64                     `yaml:",omitempty"`
	Required             []string                   `yaml:",omitempty"`
	Items                *JSONSchemaPropsOrArray    `yaml:",omitempty"`
	AllOf                []JSONSchemaProps          `yaml:",omitempty"`
	OneOf                []JSONSchemaProps          `yaml:",omitempty"`
	AnyOf                []JSONSchemaProps          `yaml:",omitempty"`
	Not                  *JSONSchemaProps           `yaml:",omitempty"`
	Properties           map[string]JSONSchemaProps `yaml:",omitempty"`
	AdditionalProperties *JSONSchemaPropsOrBool     `yaml:",omitempty"`
	PatternProperties    map[string]JSONSchemaProps `yaml:",omitempty"`
	Dependencies         JSONSchemaDependencies     `yaml:",omitempty"`
	AdditionalItems      *JSONSchemaPropsOrBool     `yaml:",omitempty"`
	Definitions          JSONSchemaDefinitions      `yaml:",omitempty"`
	ExternalDocs         *ExternalDocumentation     `yaml:",omitempty"`
	Example              *JSON                      `yaml:",omitempty"`

	// x-kubernetes-preserve-unknown-fields stops the API server
	// decoding step from pruning fields which are not specified
	// in the validation schema. This affects fields recursively,
	// but switches back to normal pruning behaviour if nested
	// properties or additionalProperties are specified in the schema.
	// This can either be true or undefined. False is forbidden.
	XPreserveUnknownFields *bool `yaml:"x-kubernetes-preserve-unknown-fields,omitempty"`

	// x-kubernetes-embedded-resource defines that the value is an
	// embedded Kubernetes runtime.Object, with TypeMeta and
	// ObjectMeta. The type must be object. It is allowed to further
	// restrict the embedded object. Both ObjectMeta and TypeMeta
	// are validated automatically. x-kubernetes-preserve-unknown-fields
	// must be true.
	XEmbeddedResource bool `yaml:"x-kubernetes-embedded-resource,omitempty"`

	// x-kubernetes-int-or-string specifies that this value is
	// either an integer or a string. If this is true, an empty
	// type is allowed and type as child of anyOf is permitted
	// if following one of the following patterns:
	//
	// 1) anyOf:
	//    - type: integer
	//    - type: string
	// 2) allOf:
	//    - anyOf:
	//      - type: integer
	//      - type: string
	//    - ... zero or more
	XIntOrString bool `yaml:"x-kubernetes-int-or-string specifies,omitempty"`

	// x-kubernetes-list-map-keys annotates an array with the x-kubernetes-list-type `map` by specifying the keys used
	// as the index of the map.
	//
	// This tag MUST only be used on lists that have the "x-kubernetes-list-type"
	// extension set to "map". Also, the values specified for this attribute must
	// be a scalar typed field of the child structure (no nesting is supported).
	XListMapKeys []string `yaml:"x-kubernetes-list-map-keys,omitempty"`

	// x-kubernetes-list-type annotates an array to further describe its topology.
	// This extension must only be used on lists and may have 3 possible values:
	//
	// 1) `atomic`: the list is treated as a single entity, like a scalar.
	//      Atomic lists will be entirely replaced when updated. This extension
	//      may be used on any type of list (struct, scalar, ...).
	// 2) `set`:
	//      Sets are lists that must not have multiple items with the same value. Each
	//      value must be a scalar, an object with x-kubernetes-map-type `atomic` or an
	//      array with x-kubernetes-list-type `atomic`.
	// 3) `map`:
	//      These lists are like maps in that their elements have a non-index key
	//      used to identify them. Order is preserved upon merge. The map tag
	//      must only be used on a list with elements of type object.
	XListType *string `yaml:"x-kubernetes-list-type,omitempty"`

	// x-kubernetes-map-type annotates an object to further describe its topology.
	// This extension must only be used when type is object and may have 2 possible values:
	//
	// 1) `granular`:
	//      These maps are actual maps (key-value pairs) and each fields are independent
	//      from each other (they can each be manipulated by separate actors). This is
	//      the default behaviour for all maps.
	// 2) `atomic`: the list is treated as a single entity, like a scalar.
	//      Atomic maps will be entirely replaced when updated.
	// +optional
	XMapType *string `yaml:"x-kubernetes-map-type,omitempty"`
}

// JSON represents any valid JSON value.
// These types are supported: bool, int64, float64, string, []interface{}, map[string]interface{} and nil.
type JSON interface{}

// JSONSchemaURL represents a schema url.
type JSONSchemaURL string

// JSONSchemaPropsOrArray represents a value that can either be a JSONSchemaProps
// or an array of JSONSchemaProps. Mainly here for serialization purposes.
type JSONSchemaPropsOrArray struct {
	Schema      *JSONSchemaProps  `yaml:",inline,omitempty"`
	JSONSchemas []JSONSchemaProps `yaml:",omitempty"`
}

// JSONSchemaPropsOrBool represents JSONSchemaProps or a boolean value.
// Defaults to true for the boolean property.
type JSONSchemaPropsOrBool struct {
	Allows bool             `yaml:",omitempty"`
	Schema *JSONSchemaProps `yaml:",omitempty"`
}

// JSONSchemaDependencies represents a dependencies property.
type JSONSchemaDependencies map[string]JSONSchemaPropsOrStringArray

// JSONSchemaPropsOrStringArray represents a JSONSchemaProps or a string array.
type JSONSchemaPropsOrStringArray struct {
	Schema   *JSONSchemaProps `yaml:",omitempty"`
	Property []string         `yaml:",omitempty"`
}

// JSONSchemaDefinitions contains the models explicitly defined in this spec.
type JSONSchemaDefinitions map[string]JSONSchemaProps

// ExternalDocumentation allows referencing an external resource for extended documentation.
type ExternalDocumentation struct {
	Description string
	URL         string
}
