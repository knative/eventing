/*
Copyright 2025 The Knative Authors

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
	appsv1 "k8s.io/api/apps/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
)

const (
	TransformConditionAddressable apis.ConditionType = "Addressable"
	TransformationConditionReady  apis.ConditionType = "TransformationReady"

	TransformationAddressableEmptyURL                   string = "NoURL"
	TransformationAddressableWaitingForServiceEndpoints string = "WaitingForServiceEndpoints"

	// Specific transformations conditions

	// TransformationJsonataDeploymentReady is the condition to indicate that the Jsonata deployment
	// is ready.
	TransformationJsonataDeploymentReady       apis.ConditionType = "JsonataDeploymentReady"
	TransformationJsonataDeploymentUnavailable string             = "JsonataDeploymentUnavailable"
	// TransformationJsonataSinkBindingReady is the condition to indicate that the Jsonata sink
	// binding is ready.
	TransformationJsonataSinkBindingReady apis.ConditionType = "JsonataSinkBindingReady"
)

var TransformCondSet = apis.NewLivingConditionSet(
	TransformationConditionReady,
	TransformConditionAddressable,
)

// transformJsonataConditionSet is the subset of conditions for the Jsonata transformation
// The overall readiness of those conditions will be propagated to the top-level
// TransformationConditionReady condition.
var transformJsonataConditionSet = apis.NewLivingConditionSet(
	TransformationJsonataDeploymentReady,
	TransformationJsonataSinkBindingReady,
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (et *EventTransform) GetConditionSet() apis.ConditionSet {
	if et == nil {
		return TransformCondSet
	}
	return et.Status.GetConditionSet()
}

func (*EventTransformStatus) GetConditionSet() apis.ConditionSet {
	return TransformCondSet
}

// GetCondition returns the condition cutsently associated with the given type, or nil.
func (ts *EventTransformStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return ts.GetConditionSet().Manage(ts).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ts *EventTransformStatus) IsReady() bool {
	return ts.GetTopLevelCondition().IsTrue()
}

// GetTopLevelCondition returns the top level Condition.
func (ts *EventTransformStatus) GetTopLevelCondition() *apis.Condition {
	return ts.GetConditionSet().Manage(ts).GetTopLevelCondition()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ts *EventTransformStatus) InitializeConditions() {
	ts.GetConditionSet().Manage(ts).InitializeConditions()
}

func (ts *EventTransformStatus) PropagateJsonataDeploymentStatus(ds appsv1.DeploymentStatus) {
	defer ts.propagateTransformJsonataReadiness()
	if ts.JsonataTransformationStatus == nil {
		ts.JsonataTransformationStatus = &JsonataEventTransformationStatus{}
	}
	ts.JsonataTransformationStatus.Deployment = ds
	if ds.Replicas > 0 && ds.Replicas == ds.AvailableReplicas {
		transformJsonataConditionSet.Manage(ts).MarkTrue(TransformationJsonataDeploymentReady)
	} else {
		transformJsonataConditionSet.Manage(ts).MarkFalse(TransformationJsonataDeploymentReady, TransformationJsonataDeploymentUnavailable, "Expected replicas: %d, available: %d", ds.Replicas, ds.AvailableReplicas)
	}
}

func (ts *EventTransformStatus) PropagateJsonataSinkBindingUnset() {
	defer ts.propagateTransformJsonataReadiness()
	transformJsonataConditionSet.Manage(ts).MarkTrue(TransformationJsonataSinkBindingReady)
}

func (ts *EventTransformStatus) PropagateJsonataSinkBindingStatus(sbs sourcesv1.SinkBindingStatus) {
	defer ts.propagateTransformJsonataReadiness()
	if ts.JsonataTransformationStatus == nil {
		ts.JsonataTransformationStatus = &JsonataEventTransformationStatus{}
	}
	ts.SourceStatus.SinkURI = sbs.SinkURI
	ts.SourceStatus.SinkAudience = sbs.SinkAudience
	ts.SourceStatus.SinkCACerts = sbs.SinkCACerts
	ts.SourceStatus.Auth = sbs.Auth

	topLevel := sbs.GetCondition(apis.ConditionReady)
	if topLevel == nil {
		transformJsonataConditionSet.Manage(ts).MarkUnknown(TransformationJsonataSinkBindingReady, "", "")
		return
	}
	if topLevel.IsTrue() {
		transformJsonataConditionSet.Manage(ts).MarkTrue(TransformationJsonataSinkBindingReady)
	} else if topLevel.IsFalse() {
		transformJsonataConditionSet.Manage(ts).MarkFalse(TransformationJsonataSinkBindingReady, topLevel.Reason, topLevel.Message)
	} else {
		transformJsonataConditionSet.Manage(ts).MarkUnknown(TransformationJsonataSinkBindingReady, topLevel.Reason, topLevel.Message)
	}
}

func (ts *EventTransformStatus) propagateTransformJsonataReadiness() {
	ts.markTransformReady(transformJsonataConditionSet)
}

func (ts *EventTransformStatus) markTransformReady(set apis.ConditionSet) {
	dCond := set.Manage(ts).GetCondition(TransformationJsonataDeploymentReady)
	sbCond := set.Manage(ts).GetCondition(TransformationJsonataSinkBindingReady)
	if !dCond.IsTrue() {
		ts.propagateTransformationConditionStatus(dCond)
		return
	}
	if !sbCond.IsTrue() {
		ts.propagateTransformationConditionStatus(sbCond)
		return
	}
	ts.propagateTransformationConditionStatus(sbCond)
	ts.propagateTransformationConditionStatus(dCond)
}

func (ts *EventTransformStatus) propagateTransformationConditionStatus(cond *apis.Condition) {
	if cond == nil {
		ts.GetConditionSet().Manage(ts).MarkUnknown(TransformationConditionReady, "", "")
	} else if cond.IsTrue() {
		ts.GetConditionSet().Manage(ts).MarkTrue(TransformationConditionReady)
	} else if cond.IsFalse() {
		ts.GetConditionSet().Manage(ts).MarkFalse(TransformationConditionReady, cond.Reason, cond.Message)
	} else {
		ts.GetConditionSet().Manage(ts).MarkUnknown(TransformationConditionReady, cond.Reason, cond.Message)
	}
}

func (ts *EventTransformStatus) MarkWaitingForServiceEndpoints() {
	ts.GetConditionSet().Manage(ts).MarkFalse(TransformConditionAddressable, TransformationAddressableWaitingForServiceEndpoints, "URL is empty")
}

func (ts *EventTransformStatus) SetAddresses(addresses ...duckv1.Addressable) {
	if len(addresses) == 0 || addresses[0].URL.IsEmpty() {
		ts.GetConditionSet().Manage(ts).MarkFalse(TransformConditionAddressable, TransformationAddressableEmptyURL, "URL is empty")
		return
	}

	ts.AddressStatus = duckv1.AddressStatus{
		Address:   &addresses[0],
		Addresses: addresses,
	}
	ts.GetConditionSet().Manage(ts).MarkTrue(TransformConditionAddressable)
}
