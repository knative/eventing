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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
)

var sbCondSet = apis.NewLivingConditionSet()

// GetGroupVersionKind returns the GroupVersionKind.
func (s *SinkBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("SinkBinding")
}

// GetUntypedSpec implements apis.HasSpec
func (c *SinkBinding) GetUntypedSpec() interface{} {
	return c.Spec
}

// GetSubject implements psbinding.Bindable
func (fb *SinkBinding) GetSubject() tracker.Reference {
	return fb.Spec.Subject
}

// GetBindingStatus implements psbinding.Bindable
func (fb *SinkBinding) GetBindingStatus() duck.BindableStatus {
	return &fb.Status
}

// SetObservedGeneration implements psbinding.BindableStatus
func (fbs *SinkBindingStatus) SetObservedGeneration(gen int64) {
	fbs.ObservedGeneration = gen
}

// InitializeConditions populates the SinkBindingStatus's conditions field
// with all of its conditions configured to Unknown.
func (fbs *SinkBindingStatus) InitializeConditions() {
	sbCondSet.Manage(fbs).InitializeConditions()
}

// MarkBindingUnavailable marks the SinkBinding's Ready condition to False with
// the provided reason and message.
func (fbs *SinkBindingStatus) MarkBindingUnavailable(reason, message string) {
	sbCondSet.Manage(fbs).MarkFalse(SinkBindingConditionReady, reason, message)
}

// MarkBindingAvailable marks the SinkBinding's Ready condition to True.
func (fbs *SinkBindingStatus) MarkBindingAvailable() {
	sbCondSet.Manage(fbs).MarkTrue(SinkBindingConditionReady)
}

// Do implements psbinding.Bindable
func (fb *SinkBinding) Do(ctx context.Context, ps *duckv1.WithPod) {
	// First undo so that we can just unconditionally append below.
	fb.Undo(ctx, ps)

	uri := GetSinkURI(ctx)
	if uri == nil {
		logging.FromContext(ctx).Error(fmt.Sprintf("No sink URI associated with context for %+v", fb))
		return
	}

	spec := ps.Spec.Template.Spec
	for i := range spec.InitContainers {
		spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
			Name:  "K_SINK",
			Value: uri.String(),
		})
	}
	for i := range spec.Containers {
		spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
			Name:  "K_SINK",
			Value: uri.String(),
		})
	}
}

func (fb *SinkBinding) Undo(ctx context.Context, ps *duckv1.WithPod) {
	spec := ps.Spec.Template.Spec
	for i, c := range spec.InitContainers {
		for j, ev := range c.Env {
			if ev.Name == "K_SINK" {
				spec.InitContainers[i].Env = append(spec.InitContainers[i].Env[:j], spec.InitContainers[i].Env[j+1:]...)
				break
			}
		}
	}
	for i, c := range spec.Containers {
		for j, ev := range c.Env {
			if ev.Name == "K_SINK" {
				spec.Containers[i].Env = append(spec.Containers[i].Env[:j], spec.Containers[i].Env[j+1:]...)
				break
			}
		}
	}
}
