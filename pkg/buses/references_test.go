/*
Copyright 2018 The Knative Authors

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

package buses_test

import (
	"fmt"
	"testing"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	referencesTestNamespace        = "test-namespace"
	referencesTestChannelName      = "test-channel"
	referencesTestSubscriptionName = "test-subscription"
)

func TestNewChannelReference(t *testing.T) {
	channel := &eventingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referencesTestChannelName,
			Namespace: referencesTestNamespace,
		},
	}
	expected := buses.ChannelReference{
		Name:      referencesTestChannelName,
		Namespace: referencesTestNamespace,
	}
	actual := buses.NewChannelReference(channel)
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "ChannelReference", expected, actual)
	}
}

func TestNewChannelReferenceFromSubscription(t *testing.T) {
	subscription := &eventingv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referencesTestSubscriptionName,
			Namespace: referencesTestNamespace,
		},
		Spec: eventingv1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				Name: referencesTestChannelName,
			},
		},
	}
	expected := buses.ChannelReference{
		Name:      referencesTestChannelName,
		Namespace: referencesTestNamespace,
	}
	actual := buses.NewChannelReferenceFromSubscription(subscription)
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "ChannelReference", expected, actual)
	}
}

func TestNewChannelReferenceFromNames(t *testing.T) {
	expected := buses.ChannelReference{
		Name:      referencesTestChannelName,
		Namespace: referencesTestNamespace,
	}
	actual := buses.NewChannelReferenceFromNames(referencesTestChannelName, referencesTestNamespace)
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "ChannelReference", expected, actual)
	}
}

func TestChannelReference_String(t *testing.T) {
	ref := buses.ChannelReference{
		Name:      referencesTestChannelName,
		Namespace: referencesTestNamespace,
	}
	expected := fmt.Sprintf("%s/%s", referencesTestNamespace, referencesTestChannelName)
	actual := ref.String()
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "ChannelReference", expected, actual)
	}
}
