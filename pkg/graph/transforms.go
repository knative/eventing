/*
Copyright 2024 The Knative Authors

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

package graph

import (
	eventingv1beta3 "knative.dev/eventing/pkg/apis/eventing/v1beta3"
)

type NoTransform struct{}

var _ Transform = NoTransform{}

func (nt NoTransform) Apply(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
	return et, tfc
}

func (nt NoTransform) Name() string {
	return "no-transform"
}

type EventTypeTransform struct {
	EventType *eventingv1beta3.EventType
}

var _ Transform = EventTypeTransform{}

func (ett EventTypeTransform) Apply(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
	return ett.EventType.DeepCopy(), tfc
}

func (ett EventTypeTransform) Name() string {
	return "eventtype-transform"
}
