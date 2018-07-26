/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package flow

import (
	//	"encoding/base64"
	//	"encoding/json"
	"fmt"
	"testing"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	flowsv1alpha1 "github.com/knative/eventing/pkg/apis/flows/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	/*
		"github.com/knative/eventing/pkg/controller/feed/resources"
		"github.com/knative/eventing/pkg/sources"
		batchv1 "k8s.io/api/batch/v1"
	*/)

/*
TODO
- initial: feed with job deadline exceeded
  reconciled: feed failure, job exists, finalizer
*/

var (
	trueVal  = true
	falseVal = false
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()
	targetURI    = "http://target.example.com"
)

const (
	targetDNS   = "myservice.mynamespace.svc.cluster.local"
	eventType   = "myeventtype"
	eventSource = "myeventsource"
	flowName    = "test-flow"
)

func init() {
	// Add types to scheme
	feedsv1alpha1.AddToScheme(scheme.Scheme)
	flowsv1alpha1.AddToScheme(scheme.Scheme)
	servingv1alpha1.AddToScheme(scheme.Scheme)
	channelsv1alpha1.AddToScheme(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new flow: adds status, action target resolved",
		InitialState: []runtime.Object{
			getNewFlow(),
		},
		ReconcileKey: "test/test-flow",
		WantPresent: []runtime.Object{
			getActionTargetResolvedFlow(),
			getNewChannel(),
		},
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		r := &reconciler{
			client:   tc.GetClient(),
			recorder: recorder,
		}
		t.Run(tc.Name, tc.Runner(t, r, r.client))
	}
}

func getActionTargetResolvedFlow() *flowsv1alpha1.Flow {
	newFlow := getNewFlow()
	newFlow.Status = flowsv1alpha1.FlowStatus{
		Conditions: []flowsv1alpha1.FlowCondition{{
			Type:   flowsv1alpha1.FlowConditionReady,
			Status: corev1.ConditionUnknown,
		}, {
			Type:    flowsv1alpha1.FlowConditionActionTargetResolved,
			Status:  corev1.ConditionTrue,
			Reason:  "ActionTargetResolved",
			Message: fmt.Sprintf("Resolved to: %q", targetURI),
		}},
	}
	return newFlow
}

func getNewFlow() *flowsv1alpha1.Flow {
	return &flowsv1alpha1.Flow{
		TypeMeta:   flowType(),
		ObjectMeta: om("test", flowName),
		Spec: flowsv1alpha1.FlowSpec{
			Action: flowsv1alpha1.FlowAction{
				TargetURI: &targetURI,
			},
			Trigger: flowsv1alpha1.EventTrigger{
				EventType:      eventType,
				Resource:       "myresource",
				Service:        "",
				Parameters:     nil,
				ParametersFrom: nil,
			},
		},
	}
}

func getNewChannel() *channelsv1alpha1.Channel {
	channel := &channelsv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om("test", flowName),
		Spec: channelsv1alpha1.ChannelSpec{
			ClusterBus: "stub",
		},
	}
	channel.ObjectMeta.OwnerReferences = append(channel.ObjectMeta.OwnerReferences, getOwnerReference())

	// selflink is not filled in when we create the object, so clear it
	channel.ObjectMeta.SelfLink = ""
	return channel
}

func flowType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: flowsv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Flow",
	}
}

func channelType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: channelsv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Channel",
	}
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func omDeleting(namespace, name string) metav1.ObjectMeta {
	om := om(namespace, name)
	om.DeletionTimestamp = &deletionTime
	return om
}

func getOwnerReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         flowsv1alpha1.SchemeGroupVersion.String(),
		Kind:               "Flow",
		Name:               flowName,
		Controller:         &falseVal,
		BlockOwnerDeletion: &falseVal,
	}
}
