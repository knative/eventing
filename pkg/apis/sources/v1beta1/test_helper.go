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

package v1beta1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type testHelper struct{}

// TestHelper contains helpers for unit tests.
var TestHelper = testHelper{}

func (t testHelper) AvailableDeployment() *appsv1.Deployment {
	d := &appsv1.Deployment{}
	d.Name = "available"
	d.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		},
	}
	return d
}

func (t testHelper) UnavailableDeployment() *appsv1.Deployment {
	d := &appsv1.Deployment{}
	d.Name = "unavailable"
	d.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionFalse,
		},
	}
	return d
}

func (t testHelper) UnknownDeployment() *appsv1.Deployment {
	d := &appsv1.Deployment{}
	d.Name = "unknown"
	d.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionUnknown,
		},
	}
	return d
}
