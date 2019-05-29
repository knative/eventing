/*
Copyright 2019 The Knative Authors

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

package v1alpha1

import (
	"context"
	"reflect"

	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	//	"github.com/knative/pkg/kmp"
	"github.com/google/go-cmp/cmp"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

func (p *Pipeline) Validate(ctx context.Context) *apis.FieldError {
	return p.Spec.Validate(ctx).ViaField("spec")
}

func (ps *PipelineSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if len(ps.Steps) == 0 {
		errs = errs.Also(apis.ErrMissingField("steps"))
	}

	for i, s := range ps.Steps {
		if e := isValidSubscriberSpec(s); e != nil {
			errs = errs.Also(apis.ErrInvalidArrayValue(s, "steps", i))
		}
	}

	if equality.Semantic.DeepEqual(ps.ChannelTemplate, ChannelTemplateSpec{}) {
		errs = errs.Also(apis.ErrMissingField("channelTemplate.channelCRD"))
		return errs
	}

	return errs
}

func isValidSubscriberSpec(s eventingv1alpha1.SubscriberSpec) *apis.FieldError {
	var errs *apis.FieldError

	fieldsSet := make([]string, 0, 0)
	if s.Ref != nil && !equality.Semantic.DeepEqual(s.Ref, &corev1.ObjectReference{}) {
		fieldsSet = append(fieldsSet, "ref")
	}
	if s.DeprecatedDNSName != nil && *s.DeprecatedDNSName != "" {
		fieldsSet = append(fieldsSet, "dnsName")
	}
	if s.URI != nil && *s.URI != "" {
		fieldsSet = append(fieldsSet, "uri")
	}
	if len(fieldsSet) == 0 {
		errs = errs.Also(apis.ErrMissingOneOf("ref", "dnsName", "uri"))
	} else if len(fieldsSet) > 1 {
		errs = errs.Also(apis.ErrMultipleOneOf(fieldsSet...))
	}

	// If Ref given, check the fields.
	if s.Ref != nil && !equality.Semantic.DeepEqual(s.Ref, &corev1.ObjectReference{}) {
		fe := isValidObjectReferenceForSubscriber(*s.Ref)
		if fe != nil {
			errs = errs.Also(fe.ViaField("ref"))
		}
	}
	return errs
}

func isValidObjectReferenceForSubscriber(f corev1.ObjectReference) *apis.FieldError {
	return checkRequiredObjectReferenceFieldsForSubscriber(f).
		Also(checkDisallowedObjectReferenceFieldsForSubscriber(f))
}

// Check the corev1.ObjectReference to make sure it has the required fields. They
// are not checked for anything more except that they are set.
func checkRequiredObjectReferenceFieldsForSubscriber(f corev1.ObjectReference) *apis.FieldError {
	var errs *apis.FieldError
	if f.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	if f.APIVersion == "" {
		errs = errs.Also(apis.ErrMissingField("apiVersion"))
	}
	if f.Kind == "" {
		errs = errs.Also(apis.ErrMissingField("kind"))
	}
	return errs
}

// Check the corev1.ObjectReference to make sure it only has the following fields set:
// Name, Kind, APIVersion
// If any other fields are set and is not the Zero value, returns an apis.FieldError
// with the fieldpaths for all those fields.
func checkDisallowedObjectReferenceFieldsForSubscriber(f corev1.ObjectReference) *apis.FieldError {
	disallowedFields := []string{}
	// See if there are any fields that have been set that should not be.
	// TODO: Hoist this kind of stuff into pkg repository.
	s := reflect.ValueOf(f)
	typeOf := s.Type()
	for i := 0; i < s.NumField(); i++ {
		field := s.Field(i)
		fieldName := typeOf.Field(i).Name
		if fieldName == "Name" || fieldName == "Kind" || fieldName == "APIVersion" {
			continue
		}
		if !cmp.Equal(field.Interface(), reflect.Zero(field.Type()).Interface()) {
			disallowedFields = append(disallowedFields, fieldName)
		}
	}
	if len(disallowedFields) > 0 {
		fe := apis.ErrDisallowedFields(disallowedFields...)
		fe.Details = "only name, apiVersion and kind are supported fields"
		return fe
	}
	return nil

}
