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
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestFullLifecycle(t *testing.T) {

	et := &EventTransform{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: EventTransformSpec{
			Sink: nil,
			Transformations: EventTransformations{
				Jsonata: &JsonataEventTransformationSpec{
					Expression: `
    {
        "specversion": "1.0",
        "id": id,
        "type": "transformed-event",
        "source": source,
        "reason": data.reason,
        "message": data.message,
        "data": $
    }
`,
				},
			},
		},
	}
	et.Status.InitializeConditions()

	topLevel := et.Status.GetCondition(apis.ConditionReady)
	transformation := et.Status.GetCondition(TransformationConditionReady)
	addressable := et.Status.GetCondition(TransformConditionAddressable)

	assert.Equal(t, corev1.ConditionUnknown, topLevel.Status)
	assert.Equal(t, corev1.ConditionUnknown, transformation.Status)
	assert.Equal(t, corev1.ConditionUnknown, addressable.Status)
	assert.Len(t, et.Status.Conditions, 3)
	assert.Equal(t, false, et.Status.IsReady())

	ds := appsv1.DeploymentStatus{
		ObservedGeneration:  202,
		Replicas:            1,
		UpdatedReplicas:     1,
		ReadyReplicas:       1,
		AvailableReplicas:   1,
		UnavailableReplicas: 0,
	}
	et.Status.PropagateJsonataDeploymentStatus(ds)

	deploymentCondition := et.Status.GetCondition(TransformationJsonataDeploymentReady)
	assert.Equal(t, corev1.ConditionTrue, deploymentCondition.Status)
	assert.Len(t, et.Status.Conditions, 4)
	assert.Equal(t, false, et.Status.IsReady())

	transformationCondition := et.Status.GetCondition(TransformationConditionReady)
	assert.Equal(t, corev1.ConditionUnknown, transformationCondition.Status, et)
	assert.Len(t, et.Status.Conditions, 4)
	assert.Equal(t, false, et.Status.IsReady())

	et.Status.PropagateJsonataSinkBindingUnset()

	transformationCondition = et.Status.GetCondition(TransformationConditionReady)
	assert.Equal(t, corev1.ConditionTrue, transformationCondition.Status, et)
	assert.Len(t, et.Status.Conditions, 5)
	assert.Equal(t, false, et.Status.IsReady())

	et.Status.SetAddresses(duckv1.Addressable{URL: apis.HTTPS("example.com")})
	addrCondition := et.Status.GetCondition(TransformConditionAddressable)
	assert.Equal(t, corev1.ConditionTrue, addrCondition.Status, et)
	assert.Len(t, et.Status.Conditions, 5)

	assert.Equal(t, true, et.Status.IsReady())

	// All conditions are ready
	for _, c := range et.Status.Conditions {
		assert.Equal(t, true, c.IsTrue(), "Unexpected condition status %#v, expected True", c)
	}
}
