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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
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
