package jsonschema

import (
	"context"
	"fmt"

	jptr "github.com/qri-io/jsonpointer"
)

// MultipleOf defines the multipleOf JSON Schema keyword
type MultipleOf float64

// NewMultipleOf allocates a new MultipleOf keyword
func NewMultipleOf() Keyword {
	return new(MultipleOf)
}

// Register implements the Keyword interface for MultipleOf
func (m *MultipleOf) Register(uri string, registry *SchemaRegistry) {}

// Resolve implements the Keyword interface for MultipleOf
func (m *MultipleOf) Resolve(pointer jptr.Pointer, uri string) *Schema {
	return nil
}

// ValidateKeyword implements the Keyword interface for MultipleOf
func (m MultipleOf) ValidateKeyword(ctx context.Context, currentState *ValidationState, data interface{}) {
	schemaDebug("[MultipleOf] Validating")
	if num, ok := data.(float64); ok {
		div := num / float64(m)
		if float64(int(div)) != div {
			currentState.AddError(data, fmt.Sprintf("must be a multiple of %f", m))
		}
	}
}

// Maximum defines the maximum JSON Schema keyword
type Maximum float64

// NewMaximum allocates a new Maximum keyword
func NewMaximum() Keyword {
	return new(Maximum)
}

// Register implements the Keyword interface for Maximum
func (m *Maximum) Register(uri string, registry *SchemaRegistry) {}

// Resolve implements the Keyword interface for Maximum
func (m *Maximum) Resolve(pointer jptr.Pointer, uri string) *Schema {
	return nil
}

// ValidateKeyword implements the Keyword interface for Maximum
func (m Maximum) ValidateKeyword(ctx context.Context, currentState *ValidationState, data interface{}) {
	schemaDebug("[Maximum] Validating")
	if num, ok := data.(float64); ok {
		if num > float64(m) {
			currentState.AddError(data, fmt.Sprintf("must be less than or equal to %f", m))
		}
	}
}

// ExclusiveMaximum defines the exclusiveMaximum JSON Schema keyword
type ExclusiveMaximum float64

// NewExclusiveMaximum allocates a new ExclusiveMaximum keyword
func NewExclusiveMaximum() Keyword {
	return new(ExclusiveMaximum)
}

// Register implements the Keyword interface for ExclusiveMaximum
func (m *ExclusiveMaximum) Register(uri string, registry *SchemaRegistry) {}

// Resolve implements the Keyword interface for ExclusiveMaximum
func (m *ExclusiveMaximum) Resolve(pointer jptr.Pointer, uri string) *Schema {
	return nil
}

// ValidateKeyword implements the Keyword interface for ExclusiveMaximum
func (m ExclusiveMaximum) ValidateKeyword(ctx context.Context, currentState *ValidationState, data interface{}) {
	schemaDebug("[ExclusiveMaximum] Validating")
	if num, ok := data.(float64); ok {
		if num >= float64(m) {
			currentState.AddError(data, fmt.Sprintf("%f must be less than %f", num, m))
		}
	}
}

// Minimum defines the minimum JSON Schema keyword
type Minimum float64

// NewMinimum allocates a new Minimum keyword
func NewMinimum() Keyword {
	return new(Minimum)
}

// Register implements the Keyword interface for Minimum
func (m *Minimum) Register(uri string, registry *SchemaRegistry) {}

// Resolve implements the Keyword interface for Minimum
func (m *Minimum) Resolve(pointer jptr.Pointer, uri string) *Schema {
	return nil
}

// ValidateKeyword implements the Keyword interface for Minimum
func (m Minimum) ValidateKeyword(ctx context.Context, currentState *ValidationState, data interface{}) {
	schemaDebug("[Minimum] Validating")
	if num, ok := data.(float64); ok {
		if num < float64(m) {
			currentState.AddError(data, fmt.Sprintf("must be less than or equal to %f", m))
		}
	}
}

// ExclusiveMinimum defines the exclusiveMinimum JSON Schema keyword
type ExclusiveMinimum float64

// NewExclusiveMinimum allocates a new ExclusiveMinimum keyword
func NewExclusiveMinimum() Keyword {
	return new(ExclusiveMinimum)
}

// Register implements the Keyword interface for ExclusiveMinimum
func (m *ExclusiveMinimum) Register(uri string, registry *SchemaRegistry) {}

// Resolve implements the Keyword interface for ExclusiveMinimum
func (m *ExclusiveMinimum) Resolve(pointer jptr.Pointer, uri string) *Schema {
	return nil
}

// ValidateKeyword implements the Keyword interface for ExclusiveMinimum
func (m ExclusiveMinimum) ValidateKeyword(ctx context.Context, currentState *ValidationState, data interface{}) {
	schemaDebug("[ExclusiveMinimum] Validating")
	if num, ok := data.(float64); ok {
		if num <= float64(m) {
			currentState.AddError(data, fmt.Sprintf("%f must be less than %f", num, m))
		}
	}
}
