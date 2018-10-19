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

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	referencesTestNamespace        = "test-namespace"
	referencesTestBusName          = "test-bus"
	referencesTestClusterBusName   = "test-clusterbus"
	referencesTestChannelName      = "test-channel"
	referencesTestSubscriptionName = "test-subscription"
)

func TestNewBusReference(t *testing.T) {
	bus := &channelsv1alpha1.Bus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referencesTestBusName,
			Namespace: referencesTestNamespace,
		},
	}
	expected := buses.BusReference{
		Name:      referencesTestBusName,
		Namespace: referencesTestNamespace,
	}
	actual := buses.NewBusReference(bus)
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "BusReference", expected, actual)
	}
}

func TestNewBusReference_ClusterBus(t *testing.T) {
	clusterBus := &channelsv1alpha1.ClusterBus{
		ObjectMeta: metav1.ObjectMeta{
			Name: referencesTestClusterBusName,
		},
	}
	expected := buses.BusReference{
		Name: referencesTestClusterBusName,
	}
	actual := buses.NewBusReference(clusterBus)
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "BusReference", expected, actual)
	}
}

func TestNewBusReferenceFromNames(t *testing.T) {
	expected := buses.BusReference{
		Name:      referencesTestBusName,
		Namespace: referencesTestNamespace,
	}
	actual := buses.NewBusReferenceFromNames(referencesTestBusName, referencesTestNamespace)
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "BusReference", expected, actual)
	}
}

func TestNewBusReferenceFromNames_ClusterBus(t *testing.T) {
	expected := buses.BusReference{
		Name: referencesTestClusterBusName,
	}
	actual := buses.NewBusReferenceFromNames(referencesTestClusterBusName, "")
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "BusReference", expected, actual)
	}
}

func TestBusReference_IsNamespaced(t *testing.T) {
	busRef := buses.BusReference{
		Name:      referencesTestBusName,
		Namespace: referencesTestNamespace,
	}
	expected := true
	actual := busRef.IsNamespaced()
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "IsNamespaced", expected, actual)
	}
}

func TestBusReference_IsNamespaced_ClusterBus(t *testing.T) {
	busRef := buses.BusReference{
		Name: referencesTestClusterBusName,
	}
	expected := false
	actual := busRef.IsNamespaced()
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "IsNamespaced", expected, actual)
	}
}

func TestBusReference_String(t *testing.T) {
	ref := buses.BusReference{
		Name:      referencesTestBusName,
		Namespace: referencesTestNamespace,
	}
	expected := fmt.Sprintf("%s/%s", referencesTestNamespace, referencesTestBusName)
	actual := ref.String()
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "BusReference", expected, actual)
	}
}

func TestBusReference_String_ClusterBus(t *testing.T) {
	ref := buses.BusReference{
		Name: referencesTestClusterBusName,
	}
	expected := referencesTestClusterBusName
	actual := ref.String()
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "BusReference", expected, actual)
	}
}

func TestNewChannelReference(t *testing.T) {
	channel := &channelsv1alpha1.Channel{
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
	subscription := &channelsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referencesTestSubscriptionName,
			Namespace: referencesTestNamespace,
		},
		Spec: channelsv1alpha1.SubscriptionSpec{
			Channel: referencesTestChannelName,
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

func TestNewSubscriptionReference(t *testing.T) {
	subscription := &channelsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referencesTestSubscriptionName,
			Namespace: referencesTestNamespace,
		},
	}
	expected := buses.SubscriptionReference{
		Name:      referencesTestSubscriptionName,
		Namespace: referencesTestNamespace,
	}
	actual := buses.NewSubscriptionReference(subscription)
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "SubscriptionReference", expected, actual)
	}
}

func TestNewSubscriptionReferenceFromNames(t *testing.T) {
	expected := buses.SubscriptionReference{
		Name:      referencesTestSubscriptionName,
		Namespace: referencesTestNamespace,
	}
	actual := buses.NewSubscriptionReferenceFromNames(referencesTestSubscriptionName, referencesTestNamespace)
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "SubscriptionReference", expected, actual)
	}
}

func TestSubscriptionReference_String(t *testing.T) {
	ref := buses.SubscriptionReference{
		Name:      referencesTestSubscriptionName,
		Namespace: referencesTestNamespace,
	}
	expected := fmt.Sprintf("%s/%s", referencesTestNamespace, referencesTestSubscriptionName)
	actual := ref.String()
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "SubscriptionReference", expected, actual)
	}
}
