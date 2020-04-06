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

package broker

import (
	"context"
	"reflect"

	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/client/clientset/versioned/scheme"
	versionedscheme "knative.dev/eventing/pkg/client/clientset/versioned/scheme"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	_ "knative.dev/eventing/pkg/client/clientset/versioned"
	_ "knative.dev/pkg/reconciler/testing"
)

const (
	testns     = "testnamespace"
	testns2    = "testnamespace2"
	testbroker = "testbroker"
)

var (
	annotationExists          = map[string]string{"eventing.knative.dev/broker.class": "ChannelBasedBroker"}
	annotationExistsAndOthers = map[string]string{
		"eventing.knative.dev/broker.class": "ChannelBasedBroker",
		"eventing.knative.dev/creator":      "system:serviceaccount:knative-eventing:eventing-controller",
	}
	annotationOthers = map[string]string{
		"otherannotation/":             "somethingelse",
		"eventing.knative.dev/creator": "system:serviceaccount:knative-eventing:eventing-controller",
	}
	patchbytes = []byte("{\"metadata\":{\"annotations\":{\"eventing.knative.dev/broker.class\":\"ChannelBasedBroker\"}}}")
)

func TestUpgrade(t *testing.T) {
	brokers := []runtime.Object{
		broker("b1", testns2, map[string]string{}),
		broker("b2", testns, annotationExists),
		broker("b3", testns, annotationExistsAndOthers),
		broker("b4", testns, annotationOthers),
	}

	ctx, cs := fakeeventingclient.With(context.Background(), brokers...)
	ctx = context.WithValue(ctx, kubeclient.Key{}, fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns2}},
	))
	var patches []clientgotesting.PatchAction
	cs.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		patches = append(patches, action.(clientgotesting.PatchAction))
		// Not important what we ret, it's just logged
		return true, broker("b1", testns, annotationExists), nil
	})
	err := Upgrade(ctx)
	if err != nil {
		t.Errorf("Failed to process namespace: %v", err)
	}
	if len(patches) != 2 {
		t.Errorf("expected 2 patches, got %d", len(patches))
	}
	// Note these get processed in different order than when doing a single namespace
	// because they are in different namespaces
	if patches[1].GetName() != "b1" {
		t.Errorf("expected patch to %q, got %q", "b1", patches[1].GetName())
	}
	if patches[1].GetPatchType() != types.MergePatchType {
		t.Errorf("expected patchtype %+v, got %+v", types.MergePatchType, patches[1].GetPatchType())
	}
	if !reflect.DeepEqual(patches[1].GetPatch(), patchbytes) {
		t.Errorf("expected patchbytes %q, got %q", string(patchbytes), string(patches[1].GetPatch()))
	}
	if patches[0].GetName() != "b4" {
		t.Errorf("expected patch to %q, got %q", "b4", patches[0].GetName())
	}
	if patches[0].GetPatchType() != types.MergePatchType {
		t.Errorf("expected patchtype to %+v, got %+v", types.MergePatchType, patches[0].GetPatchType())
	}
	if !reflect.DeepEqual(patches[0].GetPatch(), patchbytes) {
		t.Errorf("expected patchbytes %q, got %q", string(patchbytes), string(patches[0].GetPatch()))
	}
}

func TestProcessNamespace(t *testing.T) {
	brokers := []runtime.Object{
		broker("b1", testns, map[string]string{}),
		broker("b2", testns, annotationExists),
		broker("b3", testns, annotationExistsAndOthers),
		broker("b4", testns, annotationOthers),
	}

	ctx, cs := fakeeventingclient.With(context.Background(), brokers...)
	var patches []clientgotesting.PatchAction
	cs.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		patches = append(patches, action.(clientgotesting.PatchAction))
		// Not important what we ret, it's just logged
		return true, broker("b1", testns, annotationExists), nil
	})
	err := processNamespace(ctx, testns)
	if err != nil {
		t.Errorf("Failed to process namespace: %v", err)
	}
	if len(patches) != 2 {
		t.Errorf("expected 2 patches, got %d", len(patches))
	}
	if patches[0].GetName() != "b1" {
		t.Errorf("expected patch to %q, got %q", "b1", patches[0].GetName())
	}
	if patches[0].GetPatchType() != types.MergePatchType {
		t.Errorf("expected patchtype %+v, got %+v", types.MergePatchType, patches[0].GetPatchType())
	}
	if !reflect.DeepEqual(patches[0].GetPatch(), patchbytes) {
		t.Errorf("expected patchbytes %q, got %q", string(patchbytes), string(patches[0].GetPatch()))
	}
	if patches[1].GetName() != "b4" {
		t.Errorf("expected patch to %q, got %q", "b4", patches[1].GetName())
	}
	if patches[1].GetPatchType() != types.MergePatchType {
		t.Errorf("expected patchtype to %+v, got %+v", types.MergePatchType, patches[1].GetPatchType())
	}
	if !reflect.DeepEqual(patches[1].GetPatch(), patchbytes) {
		t.Errorf("expected patchbytes %q, got %q", string(patchbytes), string(patches[1].GetPatch()))
	}
}

func TestProcessBroker(t *testing.T) {
	testcases := map[string]struct {
		in          *v1alpha1.Broker
		expectPatch []byte
	}{
		"nilannotations": {
			&v1alpha1.Broker{ObjectMeta: metav1.ObjectMeta{Name: testbroker}},
			patchbytes,
		},
		"empty": {
			broker(testbroker, testns, map[string]string{}),
			patchbytes,
		},
		"existing": {
			broker(testbroker, testns, annotationExists),
			[]byte{},
		},
		"existingandothers": {
			broker(testbroker, testns, annotationExistsAndOthers),
			[]byte{},
		},
		"onlyothers": {
			broker(testbroker, testns, annotationOthers),
			patchbytes,
		},
	}

	for _, tc := range testcases {
		ctx, _ := fakeeventingclient.With(context.Background(), tc.in)
		patch, err := processBroker(ctx, *tc.in)
		if err != nil {
			t.Errorf("Failed to process broker: %v", err)
		}
		if !reflect.DeepEqual(patch, tc.expectPatch) {
			t.Errorf("Patches differ : want: %q got: %q", tc.expectPatch, patch)
		}

	}
}

func broker(name, namespace string, annotations map[string]string) *v1alpha1.Broker {
	return &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
	}
}

func init() {
	versionedscheme.AddToScheme(scheme.Scheme)
}
