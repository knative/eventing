/*
Copyright 2020 The Knative Authors.

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
	"fmt"
	"reflect"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1alpha1.ApiServerSource) into v1alpha2.ApiServerSource
func (source *ApiServerSource) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1alpha2.ApiServerSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status = v1alpha2.ApiServerSourceStatus{
			SourceStatus: duckv1.SourceStatus{
				Status:  source.Status.Status,
				SinkURI: source.Status.SinkURI,
			},
		}

		// Optionals
		if source.Spec.Sink != nil {
			var ref *duckv1.KReference
			if source.Spec.Sink.Ref != nil {
				ref = &duckv1.KReference{
					Kind:       source.Spec.Sink.Ref.Kind,
					Namespace:  source.Namespace,
					Name:       source.Spec.Sink.Ref.Name,
					APIVersion: source.Spec.Sink.Ref.APIVersion,
				}
			}
			sink.Spec.Sink = duckv1.Destination{
				Ref: ref,
				URI: source.Spec.Sink.URI,
			}
		}
		if source.Spec.CloudEventOverrides != nil {
			sink.Spec.CloudEventOverrides = source.Spec.CloudEventOverrides.DeepCopy()
		}
		if source.Status.SinkURI != nil {
			sink.Status.SinkURI = source.Status.SinkURI.DeepCopy()
		}
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1alpha2.ApiServerSource into v1alpha1.ApiServerSource
func (sink *ApiServerSource) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1alpha2.PingSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = ApiServerSourceSpec{
			//Sink:                source.Spec.Sink.DeepCopy(),
			CloudEventOverrides: source.Spec.CloudEventOverrides,
		}
		sink.Status = ApiServerSourceStatus{
			Status:  source.Status.Status,
			SinkURI: source.Status.SinkURI,
		}
		if reflect.DeepEqual(*sink.Spec.Sink, duckv1.Destination{}) {
			sink.Spec.Sink = nil
		}
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
