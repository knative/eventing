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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
)

func TestMakeEventPolicyForBackingChannel(t *testing.T) {
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-broker",
			Namespace: "test-namespace",
		},
	}
	backingChannel := &eventingduckv1.Channelable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-channel",
			Namespace: "test-namespace",
		},
	}
	want := &eventingv1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      BrokerEventPolicyName(broker.Name, backingChannel.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "test-broker",
				},
			},
			Labels: LabelsForBackingChannelsEventPolicy(broker),
		},
		Spec: eventingv1alpha1.EventPolicySpec{
			To: []eventingv1alpha1.EventPolicySpecTo{
				{
					Ref: &eventingv1alpha1.EventPolicyToReference{
						APIVersion: backingChannel.APIVersion,
						Kind:       backingChannel.Kind,
						Name:       backingChannel.Name,
					},
				},
			},
			From: []eventingv1alpha1.EventPolicySpecFrom{
				{
					Sub: ptr.To(OIDCBrokerSub),
				},
			},
		},
	}
	got := MakeEventPolicyForBackingChannel(broker, backingChannel)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("MakeEventPolicyForBackingChannel() (-want, +got) = %v", diff)
	}
}
