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

package v1beta2

import (
	"context"

	"knative.dev/pkg/apis"

	v1 "knative.dev/eventing/pkg/apis/sources/v1"
)

// ConvertTo implements apis.Convertible
// Converts source from v1beta2.PingSource into a higher version.
func (source *PingSource) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1.PingSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status = v1.PingSourceStatus{
			SourceStatus: source.Status.SourceStatus,
		}
		sink.Spec = v1.PingSourceSpec{
			SourceSpec:  source.Spec.SourceSpec,
			Schedule:    source.Spec.Schedule,
			Timezone:    source.Spec.Timezone,
			ContentType: source.Spec.ContentType,
			Data:        source.Spec.Data,
			DataBase64:  source.Spec.DataBase64,
		}

		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1.PingSource{}, sink)
	}
}

// ConvertFrom implements apis.Convertible
// Converts source from a higher version into v1beta2.PingSource
func (sink *PingSource) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1.PingSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status = PingSourceStatus{
			SourceStatus: source.Status.SourceStatus,
		}

		sink.Spec = PingSourceSpec{
			SourceSpec:  source.Spec.SourceSpec,
			Schedule:    source.Spec.Schedule,
			Timezone:    source.Spec.Timezone,
			ContentType: source.Spec.ContentType,
			Data:        source.Spec.Data,
			DataBase64:  source.Spec.DataBase64,
		}

		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1.PingSource{}, sink)
	}
}
