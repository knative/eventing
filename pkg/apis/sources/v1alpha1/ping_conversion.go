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
// Converts source (from v1alpha1.PingSource) into v1alpha2.PingSource
func (source *PingSource) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1alpha2.PingSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = v1alpha2.PingSourceSpec{
			Schedule: source.Spec.Schedule,
			JsonData: source.Spec.Data,
		}
		sink.Status = v1alpha2.PingSourceStatus{
			SourceStatus: duckv1.SourceStatus{
				Status:  source.Status.Status,
				SinkURI: source.Status.SinkURI,
			},
		}
		// Optionals
		if source.Spec.Sink != nil {
			sink.Spec.Sink = *source.Spec.Sink.DeepCopy()
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
// Converts obj from v1alpha2.PingSource into v1alpha1.PingSource
func (sink *PingSource) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1alpha2.PingSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = PingSourceSpec{
			Schedule:            source.Spec.Schedule,
			Data:                source.Spec.JsonData,
			Sink:                source.Spec.Sink.DeepCopy(),
			CloudEventOverrides: source.Spec.CloudEventOverrides,
		}
		sink.Status = PingSourceStatus{
			SourceStatus: duckv1.SourceStatus{
				Status:  source.Status.Status,
				SinkURI: source.Status.SinkURI,
			},
		}
		if reflect.DeepEqual(*sink.Spec.Sink, duckv1.Destination{}) {
			sink.Spec.Sink = nil
		}
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
