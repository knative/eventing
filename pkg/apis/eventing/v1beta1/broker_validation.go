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

package v1beta1

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
)

const (
	brokerClassAnnotationKey = "eventing.knative.dev/broker.class"
)

func (b *Broker) Validate(ctx context.Context) *apis.FieldError {
	return b.Spec.Validate(ctx).ViaField("spec")
}

func (bs *BrokerSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Validate the Config
	if bs.Config != nil {
		if ce := isValidConfig(bs.Config); ce != nil {
			errs = errs.Also(ce.ViaField("config"))
		}
	}

	if bs.Delivery != nil {
		if de := bs.Delivery.Validate(ctx); de != nil {
			errs = errs.Also(de.ViaField("delivery"))
		}
	}
	return errs
}

func isValidConfig(ref *corev1.ObjectReference) *apis.FieldError {
	if ref == nil {
		return nil
	}
	// Check the object.
	var errs *apis.FieldError
	if ref.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	if ref.Namespace == "" {
		errs = errs.Also(apis.ErrMissingField("namespace"))
	}
	if ref.APIVersion == "" {
		errs = errs.Also(apis.ErrMissingField("apiVersion"))
	}
	if ref.Kind == "" {
		errs = errs.Also(apis.ErrMissingField("kind"))
	}

	return errs
}

func (b *Broker) CheckImmutableFields(ctx context.Context, original *Broker) *apis.FieldError {
	if original == nil {
		return nil
	}

	// Make sure you can't change the class annotation.
	diff, err := kmp.ShortDiff(original.GetAnnotations()[brokerClassAnnotationKey], b.GetAnnotations()[brokerClassAnnotationKey])

	if err != nil {
		return &apis.FieldError{
			Message: "couldn't diff the Broker objects",
			Details: err.Error(),
		}
	}

	if diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"annotations"},
			Details: diff,
		}
	}
	return nil
}
