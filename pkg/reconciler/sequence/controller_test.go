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

package sequence

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/tracker"

	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	"knative.dev/eventing/pkg/apis/feature"
	_ "knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/flows/v1/sequence/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	c := NewController(ctx, testConfigMapWatcher())

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func testConfigMapWatcher() configmap.Watcher {
	return configmap.NewStaticWatcher(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      feature.FlagsConfigName,
			Namespace: "knative-eventing",
		},
	})
}

func TestSequenceDeleteRemovesTrackerObservers(t *testing.T) {
	ctx, cancel, informers := SetupFakeContextWithCancel(t)
	defer cancel()

	impl := NewController(ctx, testConfigMapWatcher())

	obj := &flowsv1.Sequence{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "obj"},
	}
	ref := tracker.Reference{
		APIVersion: "messaging.knative.dev/v1",
		Kind:       "InMemoryChannel",
		Namespace:  "ns",
		Name:       "ch",
	}
	if err := impl.Tracker.TrackReference(ref, obj); err != nil {
		t.Fatal("TrackReference() =", err)
	}

	tracked := &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: "messaging.knative.dev/v1", Kind: "InMemoryChannel"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "ch"},
	}
	if got := impl.Tracker.GetObservers(tracked); len(got) != 1 {
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

	client := fakeeventingclient.Get(ctx).FlowsV1().Sequences("ns")
	if _, err := client.Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		t.Fatal("Create() =", err)
	}
	if err := client.Delete(ctx, obj.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal("Delete() =", err)
	}

	if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
		return len(impl.Tracker.GetObservers(tracked)) == 0, nil
	}); err != nil {
		t.Error("Tracker still has observers after Sequence deletion:", impl.Tracker.GetObservers(tracked))
	}
}
