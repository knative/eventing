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

	"github.com/knative/eventing/pkg/apis/eventing"
	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (t *Trigger) SetDefaults(ctx context.Context) {
	t.Spec.SetDefaults(ctx)

	if ui := apis.GetUserInfo(ctx); ui != nil {
		ans := t.GetAnnotations()
		if ans == nil {
			ans = map[string]string{}
			defer t.SetAnnotations(ans)
		}

		if apis.IsInUpdate(ctx) {
			old := apis.GetBaseline(ctx).(*Trigger)
			if equality.Semantic.DeepEqual(old.Spec, t.Spec) {
				return
			}
			ans[eventing.UpdaterAnnotation] = ui.Username
		} else {
			ans[eventing.CreatorAnnotation] = ui.Username
			ans[eventing.UpdaterAnnotation] = ui.Username
		}
	}
}

func (ts *TriggerSpec) SetDefaults(ctx context.Context) {
	if ts.Broker == "" {
		ts.Broker = "default"
	}
	// Make a default filter that allows anything.
	if ts.Filter == nil {
		ts.Filter = &TriggerFilter{}
	}

	// Note that this logic will need to change once there are other filtering options, as it should
	// only apply if no other filter is applied.
	if ts.Filter.SourceAndType == nil {
		ts.Filter.SourceAndType = &TriggerFilterSourceAndType{}
	}
	if ts.Filter.SourceAndType.Type == "" {
		ts.Filter.SourceAndType.Type = TriggerAnyFilter
	}
	if ts.Filter.SourceAndType.Source == "" {
		ts.Filter.SourceAndType.Source = TriggerAnyFilter
	}
}
