package test

import (
	"fmt"
	"reflect"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/event"
)

type EventMatcher func(have event.Event) error

// AllOf combines matchers together
func AllOf(matchers ...EventMatcher) EventMatcher {
	return func(have event.Event) error {
		for _, m := range matchers {
			if err := m(have); err != nil {
				return err
			}
		}
		return nil
	}
}

func HasId(id string) EventMatcher {
	return HasAttributeKind(spec.ID, id)
}

func HasType(ty string) EventMatcher {
	return HasAttributeKind(spec.Type, ty)
}

func HasSpecVersion(specVersion string) EventMatcher {
	return HasAttributeKind(spec.SpecVersion, specVersion)
}

func HasSource(source string) EventMatcher {
	return HasAttributeKind(spec.Source, source)
}

func HasDataContentType(dataContentType string) EventMatcher {
	return HasAttributeKind(spec.DataContentType, dataContentType)
}

func HasDataSchema(schema string) EventMatcher {
	return HasAttributeKind(spec.DataSchema, schema)
}

func HasSubject(subject string) EventMatcher {
	return HasAttributeKind(spec.Subject, subject)
}

func HasTime(t time.Time) EventMatcher {
	return HasAttributeKind(spec.Time, t)
}

// ContainsAttributes checks if the event contains at least the provided context attributes
func ContainsAttributes(attrs ...spec.Kind) EventMatcher {
	return func(have event.Event) error {
		haveVersion := spec.VS.Version(have.SpecVersion())
		for _, k := range attrs {
			attr := haveVersion.AttributeFromKind(k)
			if isEmpty(attr) {
				return fmt.Errorf("attribute name '%s' unrecognized", k.String())
			}
			if isEmpty(attr.Get(have.Context)) {
				return fmt.Errorf("missing or nil/empty attribute '%s'", k.String())
			}
		}
		return nil
	}
}

// ContainsExtensions checks if the event contains at least the provided extension names
func ContainsExtensions(exts ...string) EventMatcher {
	return func(have event.Event) error {
		for _, ext := range exts {
			if _, ok := have.Extensions()[ext]; !ok {
				return fmt.Errorf("expecting extension '%s'", ext)
			}
		}
		return nil
	}
}

// HasExactlyExtensions checks if the event contains exactly the provided extensions
func HasExactlyExtensions(ext map[string]interface{}) EventMatcher {
	return func(have event.Event) error {
		if diff := cmp.Diff(ext, have.Extensions()); diff != "" {
			return fmt.Errorf("unexpected extensions (-want, +got) = %v", diff)
		}
		return nil
	}
}

// HasExtensions checks if the event contains at least the provided extensions
func HasExtensions(ext map[string]interface{}) EventMatcher {
	return func(have event.Event) error {
		for k, v := range ext {
			if _, ok := have.Extensions()[k]; !ok {
				return fmt.Errorf("expecting extension '%s'", ext)
			}
			if !reflect.DeepEqual(v, have.Extensions()[k]) {
				return fmt.Errorf("expecting extension '%s' equal to '%s', got '%s'", k, v, have.Extensions()[k])
			}
		}
		return nil
	}
}

// HasExtension checks if the event contains the provided extension
func HasExtension(key string, value interface{}) EventMatcher {
	return HasExtensions(map[string]interface{}{key: value})
}

// HasData checks if the event contains the provided data
func HasData(want []byte) EventMatcher {
	return func(have event.Event) error {
		if diff := cmp.Diff(string(want), string(have.Data())); diff != "" {
			return fmt.Errorf("data not matching (-want, +got) = %v", diff)
		}
		return nil
	}
}

// HasNoData checks if the event doesn't contain data
func HasNoData() EventMatcher {
	return func(have event.Event) error {
		if have.Data() != nil {
			return fmt.Errorf("expecting nil data, got = '%v'", string(have.Data()))
		}
		return nil
	}
}

// IsEqualTo performs a semantic equality check of the event (like AssertEventEquals)
func IsEqualTo(want event.Event) EventMatcher {
	return AllOf(IsContextEqualTo(want.Context), IsDataEqualTo(want))
}

// IsContextEqualTo performs a semantic equality check of the event context (like AssertEventContextEquals)
func IsContextEqualTo(want event.EventContext) EventMatcher {
	return AllOf(func(have event.Event) error {
		if want.GetSpecVersion() != have.SpecVersion() {
			return fmt.Errorf("not matching specversion: want = '%s', got = '%s'", want.GetSpecVersion(), have.SpecVersion())
		}
		vs := spec.VS.Version(want.GetSpecVersion())

		for _, a := range vs.Attributes() {
			if !reflect.DeepEqual(a.Get(want), a.Get(have.Context)) {
				return fmt.Errorf("expecting attribute '%s' equal to '%s', got '%s'", a.PrefixedName(), a.Get(want), a.Get(have.Context))
			}
		}

		return nil
	}, HasExactlyExtensions(want.GetExtensions()))
}

// IsDataEqualTo checks if the data field matches with want
func IsDataEqualTo(want event.Event) EventMatcher {
	if want.Data() == nil {
		return HasNoData()
	} else {
		return HasData(want.Data())
	}
}

// IsValid checks if the event is valid
func IsValid() EventMatcher {
	return func(have event.Event) error {
		if err := have.Validate(); err != nil {
			return fmt.Errorf("expecting valid event: %s", err.Error())
		}
		return nil
	}
}

// IsInvalid checks if the event is invalid
func IsInvalid() EventMatcher {
	return func(have event.Event) error {
		if err := have.Validate(); err == nil {
			return fmt.Errorf("expecting invalid event")
		}
		return nil
	}
}

func HasAttributeKind(kind spec.Kind, value interface{}) EventMatcher {
	return func(have event.Event) error {
		haveVersion := spec.VS.Version(have.SpecVersion())
		attr := haveVersion.AttributeFromKind(kind)
		if isEmpty(attr) {
			return fmt.Errorf("attribute '%s' not existing in the spec version '%s' of this event", kind.String(), haveVersion.String())
		}
		if !reflect.DeepEqual(value, attr.Get(have.Context)) {
			return fmt.Errorf("expecting attribute '%s' equal to '%s', got '%s'", kind.String(), value, attr.Get(have.Context))
		}
		return nil
	}
}

// Code took from https://github.com/stretchr/testify
// LICENSE: MIT License

func isEmpty(object interface{}) bool {

	// get nil case out of the way
	if object == nil {
		return true
	}

	objValue := reflect.ValueOf(object)

	switch objValue.Kind() {
	// collection types are empty when they have no element
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		return objValue.Len() == 0
		// pointers are empty if nil or if the value they point to is empty
	case reflect.Ptr:
		if objValue.IsNil() {
			return true
		}
		deref := objValue.Elem().Interface()
		return isEmpty(deref)
		// for all other types, compare against the zero value
	default:
		zero := reflect.Zero(objValue.Type())
		return reflect.DeepEqual(object, zero.Interface())
	}
}
