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

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1alpha1.SinkBinding) into v1alpha2.SinkBinding
func (source *SinkBinding) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1alpha2.SinkBinding:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.SourceSpec = source.Spec.SourceSpec
		sink.Spec.BindingSpec = source.Spec.BindingSpec
		sink.Status.SourceStatus = source.Status.SourceStatus
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1alpha2.SinkBinding into v1alpha1.SinkBinding
func (sink *SinkBinding) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1alpha2.SinkBinding:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.SourceSpec = source.Spec.SourceSpec
		sink.Spec.BindingSpec = source.Spec.BindingSpec
		sink.Status.SourceStatus = source.Status.SourceStatus
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
