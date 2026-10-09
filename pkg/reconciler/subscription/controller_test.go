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

package subscription

import (
	"context"
	"testing"
	"time"

	"knative.dev/eventing/pkg/auth"
	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/apis/feature"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/channel/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription/fake"
	_ "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1/customresourcedefinition/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/factory/filtered/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t, SetUpInformerSelector)

	c := NewController(ctx, configmap.NewStaticWatcher(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: feature.FlagsConfigName,
			},
		},
	))

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func SetUpInformerSelector(ctx context.Context) context.Context {
	ctx = filteredFactory.WithSelectors(ctx, auth.OIDCLabelSelector)
	return ctx
}

func TestSubscriptionDeleteRemovesTrackerObservers(t *testing.T) {
	ctx, cancel, informers := SetupFakeContextWithCancel(t, SetUpInformerSelector)
	defer cancel()

	impl := NewController(ctx, configmap.NewStaticWatcher(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: feature.FlagsConfigName,
			},
		},
	))

	sub := &messagingv1.Subscription{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sub"},
	}
	channelRef := tracker.Reference{
		APIVersion: "messaging.knative.dev/v1",
		Kind:       "Channel",
		Namespace:  "ns",
		Name:       "ch",
	}
	if err := impl.Tracker.TrackReference(channelRef, sub); err != nil {
		t.Fatal("TrackReference() =", err)
	}

	channel := &messagingv1.Channel{
		TypeMeta:   metav1.TypeMeta{APIVersion: "messaging.knative.dev/v1", Kind: "Channel"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "ch"},
	}
	if got := impl.Tracker.GetObservers(channel); len(got) != 1 {
		t.Fatalf("GetObservers() = %v, want 1 observer", got)
	}

	waitInformers, err := RunAndSyncInformers(ctx, informers...)
	if err != nil {
		t.Fatal("RunAndSyncInformers() =", err)
	}
	defer func() {
		cancel()
		waitInformers()
	}()

	subs := fakeeventingclient.Get(ctx).MessagingV1().Subscriptions("ns")
	if _, err := subs.Create(ctx, sub, metav1.CreateOptions{}); err != nil {
		t.Fatal("Create() =", err)
	}
	if err := subs.Delete(ctx, sub.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal("Delete() =", err)
	}

	if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
		return len(impl.Tracker.GetObservers(channel)) == 0, nil
	}); err != nil {
		t.Error("Tracker still has observers after Subscription deletion:", impl.Tracker.GetObservers(channel))
	}
}
