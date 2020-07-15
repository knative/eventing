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

package v1alpha2

import (
	"context"

	"knative.dev/eventing/pkg/apis/sources/v1beta1"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// ConvertTo implements apis.Convertible.
// Converts source from v1alpha2.SinkBinding into a higher version.
func (source *SinkBinding) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.SinkBinding:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.SourceSpec = source.Spec.SourceSpec
		sink.Spec.BindingSpec = duckv1beta1.BindingSpec{
			Subject: source.Spec.BindingSpec.Subject,
		}
		sink.Status.SourceStatus = source.Status.SourceStatus
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1beta1.SinkBinding{}, sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from a higher version into v1alpha2.SinkBinding.
func (sink *SinkBinding) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.SinkBinding:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.SourceSpec = source.Spec.SourceSpec
		sink.Spec.BindingSpec = duckv1alpha1.BindingSpec{
			Subject: source.Spec.BindingSpec.Subject,
		}
		sink.Status.SourceStatus = source.Status.SourceStatus
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1beta1.SinkBinding{}, sink)
	}
}
