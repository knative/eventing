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
	"fmt"
	"reflect"
	"strings"

	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/eventing/pkg/client/clientset/versioned/scheme"
	versionedscheme "knative.dev/eventing/pkg/client/clientset/versioned/scheme"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/kmeta"

	_ "knative.dev/eventing/pkg/client/clientset/versioned"
	_ "knative.dev/pkg/reconciler/testing"
)

const (
	testns     = "testnamespace"
	testns2    = "testnamespace2"
	testbroker = "testbroker"
	imcSpec    = `
apiVersion: "messaging.knative.dev/v1alpha1"
kind: "InMemoryChannel"
`
	kafkaSpec = `
apiVersion: "messaging.knative.dev/v1alpha1"
kind: "KafkaChannel"
spec:
  numPartitions: 3
  replicationFactor: 1
`

	patchbytesFmt = "{\"spec\":{\"channelTemplateSpec\":null,\"config\":{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"name\":\"broker-upgrade-auto-gen-config-%s\",\"namespace\":\"%s\"}}}"
)

var (
	noPatch = []byte{}

	imc = &messagingv1beta1.ChannelTemplateSpec{
		TypeMeta: v1.TypeMeta{
			Kind:       "InMemoryChannel",
			APIVersion: "messaging.knative.dev/v1alpha1",
		},
	}
	kafka = &messagingv1beta1.ChannelTemplateSpec{
		TypeMeta: v1.TypeMeta{
			Kind:       "KafkaChannel",
			APIVersion: "messaging.knative.dev/v1alpha1",
		},
		Spec: &runtime.RawExtension{
			Raw: []byte(`{"numPartitions": 3, "replicationFactor": 1}`),
		},
	}
	ignoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())

	safeDeployDiff = cmpopts.IgnoreUnexported(resource.Quantity{})
)

func TestUpgrade(t *testing.T) {
	brokers := []runtime.Object{
		broker("b1", testns2, imc, nil),
		broker("b2", testns, nil, config("b2", testns)),
		broker("b3", testns, kafka, nil),
		broker("b4", testns, nil, config("b4", testns)),
	}

	ctx, cs := fakeeventingclient.With(context.Background(), brokers...)
	kc := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns2}},
	)

	ctx = context.WithValue(ctx, kubeclient.Key{}, kc)

	var creates []*corev1.ConfigMap
	kc.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		creates = append(creates, action.(clientgotesting.CreateAction).GetObject().(*corev1.ConfigMap))
		// Not important what we ret, it's just logged
		return true, &corev1.ConfigMap{}, nil
	})

	var patches []clientgotesting.PatchAction
	cs.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		patches = append(patches, action.(clientgotesting.PatchAction))
		// Not important what we ret, it's just logged
		return true, broker("b1", testns, nil, nil), nil
	})
	err := Upgrade(ctx)
	if err != nil {
		t.Errorf("Failed to process namespace: %v", err)
	}
	checkPatches(t, []string{"b3", "b1"}, patches, [][]byte{patchbytes("b3", testns), patchbytes("b1", testns2)})
	checkCreates(t, creates, []*corev1.ConfigMap{configMap("b3", testns, kafkaSpec), configMap("b1", testns2, imcSpec)})

}

func TestUpgradeListFails(t *testing.T) {
	brokers := []runtime.Object{
		broker("b1", testns2, imc, nil),
	}

	ctx, _ := fakeeventingclient.With(context.Background(), brokers...)
	kc := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns2}},
	)

	ctx = context.WithValue(ctx, kubeclient.Key{}, kc)
	kc.PrependReactor("list", "namespaces", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("inducing failure for %s %s", action.GetVerb(), action.GetResource().Resource)
	})
	err := Upgrade(ctx)
	if err == nil {
		t.Errorf("processNamespace did not fail")
	}

}

func TestProcessNamespace(t *testing.T) {
	brokers := []runtime.Object{
		broker("b1", testns, imc, nil),
		broker("b2", testns, nil, config("b2", testns)),
		broker("b3", testns, kafka, nil),
		broker("b4", testns, nil, config("b4", testns)),
	}

	ctx, cs := fakeeventingclient.With(context.Background(), brokers...)
	kc := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns2}},
	)

	ctx = context.WithValue(ctx, kubeclient.Key{}, kc)
	var creates []*corev1.ConfigMap
	kc.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		creates = append(creates, action.(clientgotesting.CreateAction).GetObject().(*corev1.ConfigMap))
		// Not important what we ret, it's just logged
		return true, &corev1.ConfigMap{}, nil
	})

	var patches []clientgotesting.PatchAction
	cs.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		patches = append(patches, action.(clientgotesting.PatchAction))
		// Not important what we ret, it's just logged
		return true, broker("b1", testns, nil, nil), nil
	})
	err := processNamespace(ctx, testns)
	if err != nil {
		t.Errorf("processNamespace failed: %s", err)
	}
	checkPatches(t, []string{"b1", "b3"}, patches, [][]byte{patchbytes("b1", testns), patchbytes("b3", testns)})
	checkCreates(t, creates, []*corev1.ConfigMap{configMap("b1", testns, imcSpec), configMap("b3", testns, kafkaSpec)})
}

func TestProcessNamespaceListFails(t *testing.T) {
	brokers := []runtime.Object{
		broker("b1", testns2, imc, nil),
		broker("b2", testns, nil, config("b2", testns)),
		broker("b3", testns, kafka, nil),
		broker("b4", testns, nil, config("b4", testns)),
	}

	ctx, cs := fakeeventingclient.With(context.Background(), brokers...)
	kc := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns2}},
	)

	ctx = context.WithValue(ctx, kubeclient.Key{}, kc)
	cs.PrependReactor("list", "brokers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("inducing failure for %s %s", action.GetVerb(), action.GetResource().Resource)
	})
	err := processNamespace(ctx, testns)
	if err == nil {
		t.Errorf("processNamespace did not fail")
	}
}

func TestProcessNamespacePatchFails(t *testing.T) {
	brokers := []runtime.Object{
		broker("b1", testns2, imc, nil),
		broker("b2", testns, nil, config("b2", testns)),
		broker("b3", testns, kafka, nil),
		broker("b4", testns, nil, config("b4", testns)),
	}

	ctx, cs := fakeeventingclient.With(context.Background(), brokers...)
	kc := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns2}},
	)

	ctx = context.WithValue(ctx, kubeclient.Key{}, kc)
	cs.PrependReactor("patch", "brokers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("inducing failure for %s %s", action.GetVerb(), action.GetResource().Resource)
	})
	err := processNamespace(ctx, testns)
	if err == nil {
		t.Errorf("processNamespace did not fail")
	}
}

func TestUpgradeCreateConfigMapFails(t *testing.T) {
	brokers := []runtime.Object{
		broker("b1", testns2, imc, nil),
		broker("b2", testns, nil, config("b2", testns)),
		broker("b3", testns, kafka, nil),
		broker("b4", testns, nil, config("b4", testns)),
	}

	ctx, cs := fakeeventingclient.With(context.Background(), brokers...)
	kc := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns2}},
	)

	ctx = context.WithValue(ctx, kubeclient.Key{}, kc)
	var creates []*corev1.ConfigMap
	kc.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		creates = append(creates, action.(clientgotesting.CreateAction).GetObject().(*corev1.ConfigMap))
		// Not important what we ret, it's just logged
		return true, &corev1.ConfigMap{}, nil
	})
	kc.PrependReactor("create", "configmaps", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.(clientgotesting.CreateAction).GetNamespace() == testns2 {
			return true, nil, fmt.Errorf("inducing failure for %s %s", action.GetVerb(), action.GetResource().Resource)
		}
		return false, nil, nil
	})
	var patches []clientgotesting.PatchAction
	cs.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		patches = append(patches, action.(clientgotesting.PatchAction))
		// Not important what we ret, it's just logged
		return true, broker("b1", testns, nil, nil), nil
	})
	err := Upgrade(ctx)
	if err == nil {
		t.Errorf("Upgrade did not fail")
	}
	checkPatches(t, []string{"b3"}, patches, [][]byte{patchbytes("b3", testns)})
	checkCreates(t, creates, []*corev1.ConfigMap{configMap("b3", testns, kafkaSpec)})
}

func TestProcessBroker(t *testing.T) {
	testcases := map[string]struct {
		in          *v1alpha1.Broker
		expectPatch []byte
		wantCreate  []*corev1.ConfigMap
	}{
		"nilchanneltemplate": {
			&v1alpha1.Broker{ObjectMeta: metav1.ObjectMeta{Name: testbroker}},
			noPatch,
			nil,
		},
		"empty": {
			broker(testbroker, testns, &messagingv1beta1.ChannelTemplateSpec{}, nil),
			noPatch,
			nil,
		},
		"imc, create and nil out channel template": {
			broker(testbroker, testns, imc, nil),
			patchbytes(testbroker, testns),
			[]*corev1.ConfigMap{configMap(testbroker, testns, imcSpec)},
		},
		"kafka, create and nil out channel template": {
			broker(testbroker, testns, kafka, nil),
			patchbytes(testbroker, testns),
			[]*corev1.ConfigMap{configMap(testbroker, testns, kafkaSpec)},
		},
		"imc + existing config, only nil out channel template": {
			broker(testbroker, testns, imc, config(testbroker, testns)),
			[]byte("{\"spec\":{\"channelTemplateSpec\":null}}"),
			nil,
		},
		"imc config, only nil out channel template": {
			broker(testbroker, testns, nil, config(testbroker, testns)),
			noPatch,
			nil,
		},
	}

	for _, tc := range testcases {
		ctx, _ := fakeeventingclient.With(context.Background(), tc.in)
		kc := fake.NewSimpleClientset(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testns2}},
		)
		ctx = context.WithValue(ctx, kubeclient.Key{}, kc)
		var creates []*corev1.ConfigMap
		kc.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			creates = append(creates, action.(clientgotesting.CreateAction).GetObject().(*corev1.ConfigMap))
			// Not important what we ret, it's just logged
			return true, &corev1.ConfigMap{}, nil
		})

		patch, err := processBroker(ctx, *tc.in)
		if err != nil {
			t.Errorf("Failed to process broker: %v", err)
		}
		if !reflect.DeepEqual(patch, tc.expectPatch) {
			t.Errorf("Patches differ : want: %q got: %q", tc.expectPatch, patch)
		}
		checkCreates(t, creates, tc.wantCreate)
	}
}

func broker(name, namespace string, template *messagingv1beta1.ChannelTemplateSpec, config *duckv1.KReference) *v1alpha1.Broker {
	return &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.BrokerSpec{
			ChannelTemplate: template,
			Config:          config,
		},
	}
}

func config(name, namespace string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "ConfigMap",
		APIVersion: "v1",
		Name:       "broker-upgrade-auto-gen-config-" + name,
		Namespace:  namespace,
	}

}

func configMap(name, namespace string, spec string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "broker-upgrade-auto-gen-config-" + name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(&v1alpha1.Broker{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Broker",
						APIVersion: "eventing.knative.dev/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
				}),
			},
		},
		Data: map[string]string{"channelTemplateSpec": spec},
	}

}

func patchbytes(name, namespace string) []byte {
	return []byte(fmt.Sprintf(patchbytesFmt, name, namespace))
}

func init() {
	versionedscheme.AddToScheme(scheme.Scheme)
}

func checkCreates(t *testing.T, creates []*corev1.ConfigMap, want []*corev1.ConfigMap) {
	if diff := cmp.Diff(want, creates, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Unexpected configmap differences: %s", diff)
	}
}

func checkPatches(t *testing.T, names []string, patches []clientgotesting.PatchAction, want [][]byte) {
	if len(want) != len(patches) {
		t.Errorf("mismatch in number of expectations, want %d got %d", len(want), len(patches))
		return
	}
	for i, p := range patches {
		if p.GetPatchType() != types.MergePatchType {
			t.Errorf("expected patchtype %+v, got %+v", types.MergePatchType, p.GetPatchType())
		}
		if p.GetName() != names[i] {
			t.Errorf("expected patch to %q, got %q", names[i], p.GetName())
		}
		if diff := cmp.Diff(want[i], p.GetPatch(), safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Unexpected patch differences: %s", diff)
		}
	}
}
