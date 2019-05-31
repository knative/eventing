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
	//	"context"
	"testing"

	//	"github.com/knative/eventing/pkg/apis/messaging"
	"github.com/knative/pkg/apis"

	"github.com/google/go-cmp/cmp"
	duckv1beta1 "github.com/knative/pkg/apis/duck/v1beta1"
	//	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
)

var pipelineConditionReady = apis.Condition{
	Type:   PipelineConditionReady,
	Status: corev1.ConditionTrue,
}

var pipelineConditionChannelsReady = apis.Condition{
	Type:   PipelineConditionChannelsReady,
	Status: corev1.ConditionFalse,
}

var pipelineSubscriptionsReady = apis.Condition{
	Type:   PipelineConditionSubscriptionsReady,
	Status: corev1.ConditionTrue,
}

func TestPipelineGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ss        *PipelineStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		ss: &PipelineStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					pipelineConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &pipelineConditionReady,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ss.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}
