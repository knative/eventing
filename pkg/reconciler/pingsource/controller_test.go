/*
Copyright 2021 The Knative Authors

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

package pingsource

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/pkg/apis/feature"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/auth"
	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta2/eventtype/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/sources/v1/pingsource/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/factory/filtered/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding/fake"
	. "knative.dev/pkg/reconciler/testing"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t, SetUpInformerSelector)
	c := NewController(ctx, testConfigMapWatcher())

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func testConfigMapWatcher() configmap.Watcher {
	return configmap.NewStaticWatcher(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o11yconfigmap.Name(),
				Namespace: "knative-eventing",
			},
			Data: map[string]string{
				"_example": "test-config",
			},
		}, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      logging.ConfigMapName(),
				Namespace: "knative-eventing",
			},
			Data: map[string]string{
				"zap-logger-config":   "test-config",
				"loglevel.controller": "info",
				"loglevel.webhook":    "info",
			},
		}, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      feature.FlagsConfigName,
				Namespace: "knative-eventing",
			},
			Data: map[string]string{
				"_example": "test-config",
			},
		}, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "config-kreference-mapping",
				Namespace: "knative-eventing",
			},
			Data: map[string]string{
				"_example": "test-config",
			},
		},
	)
}

func SetUpInformerSelector(ctx context.Context) context.Context {
	ctx = filteredFactory.WithSelectors(ctx, auth.OIDCLabelSelector)
	return ctx
}

func TestPingSourceDeleteRemovesTrackerObservers(t *testing.T) {
	ctx, cancel, informers := SetupFakeContextWithCancel(t, SetUpInformerSelector)
	defer cancel()

	impl := NewController(ctx, testConfigMapWatcher())

	src := &sourcesv1.PingSource{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "src"},
	}
	sinkRef := tracker.Reference{
		APIVersion: "serving.knative.dev/v1",
		Kind:       "Service",
		Namespace:  "ns",
		Name:       "sink",
	}
	if err := impl.Tracker.TrackReference(sinkRef, src); err != nil {
		t.Fatal("TrackReference() =", err)
	}

	sink := &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: "serving.knative.dev/v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sink"},
	}
	if got := impl.Tracker.GetObservers(sink); len(got) != 1 {
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

	srcs := fakeeventingclient.Get(ctx).SourcesV1().PingSources("ns")
	if _, err := srcs.Create(ctx, src, metav1.CreateOptions{}); err != nil {
		t.Fatal("Create() =", err)
	}
	if err := srcs.Delete(ctx, src.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal("Delete() =", err)
	}

	if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
		return len(impl.Tracker.GetObservers(sink)) == 0, nil
	}); err != nil {
		t.Error("Tracker still has observers after PingSource deletion:", impl.Tracker.GetObservers(sink))
	}
}
