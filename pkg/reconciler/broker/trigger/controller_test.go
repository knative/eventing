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

package mttrigger

import (
	"context"
	"fmt"
	"testing"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	triggerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger"

	"knative.dev/eventing/pkg/auth"
	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/configmap"
	logtesting "knative.dev/pkg/logging/testing"

	apiseventing "knative.dev/eventing/pkg/apis/eventing"
	eventing "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	v1lister "knative.dev/eventing/pkg/client/listers/eventing/v1"
	testingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"

	. "knative.dev/pkg/reconciler/testing"

	_ "knative.dev/pkg/client/injection/ducks/duck/v1/source/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	_ "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret/fake"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/factory/filtered/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t, SetUpInformerSelector)

	c := NewController(ctx, configmap.NewStaticWatcher(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config-features"}}))

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func SetUpInformerSelector(ctx context.Context) context.Context {
	ctx = filteredFactory.WithSelectors(ctx, auth.OIDCLabelSelector)
	return ctx
}

func TestFilterOIDCServiceAccounts(t *testing.T) {
	ctx, _ := SetupFakeContext(t, SetUpInformerSelector)

	tt := []struct {
		name    string
		sa      *corev1.ServiceAccount
		trigger *eventing.Trigger
		brokers []*eventing.Broker
		pass    bool
	}{{
		name: "matching owner reference",
		sa: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "sa",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: eventing.SchemeGroupVersion.String(),
						Kind:       "Trigger",
						Name:       "tr",
						Controller: ptr.Bool(true),
					},
				},
			},
		},
		trigger: &eventing.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "tr",
			},
			Spec: eventing.TriggerSpec{
				Broker: "br",
			},
		},
		brokers: []*eventing.Broker{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "br",
				Annotations: map[string]string{
					eventing.BrokerClassAnnotationKey: apiseventing.MTChannelBrokerClassValue,
				},
			},
		}},
		pass: true,
	}, {
		name: "references trigger for wrong broker class",
		sa: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "sa",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: eventing.SchemeGroupVersion.String(),
						Kind:       "Trigger",
						Name:       "tr",
						Controller: ptr.Bool(true),
					},
				},
			},
		},
		trigger: &eventing.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "tr",
			},
			Spec: eventing.TriggerSpec{
				Broker: "br",
			},
		},
		brokers: []*eventing.Broker{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "br",
				Annotations: map[string]string{
					eventing.BrokerClassAnnotationKey: "another-broker-class",
				},
			},
		}},
		pass: false,
	}, {
		name: "no owner reference",
		sa: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "sa",
			},
		},
		trigger: &eventing.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "tr",
			},
			Spec: eventing.TriggerSpec{
				Broker: "br",
			},
		},
		brokers: []*eventing.Broker{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "br",
				Annotations: map[string]string{
					eventing.BrokerClassAnnotationKey: apiseventing.MTChannelBrokerClassValue,
				},
			},
		}},
		pass: false,
	}}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			brokerInformer := brokerinformer.Get(ctx)
			for _, obj := range tc.brokers {
				err := brokerInformer.Informer().GetStore().Add(obj)
				assert.NoError(t, err)
			}

			triggerInformer := triggerinformer.Get(ctx)
			err := triggerInformer.Informer().GetStore().Add(tc.trigger)
			assert.NoError(t, err)

			featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"))
			filter := filterOIDCServiceAccounts(featureStore, triggerInformer.Lister(), brokerInformer.Lister())
			pass := filter(tc.sa)
			assert.Equal(t, tc.pass, pass)
		})
	}
}

func TestFilterTriggers(t *testing.T) {
	ctx, _ := SetupFakeContext(t, SetUpInformerSelector)

	tt := []struct {
		name    string
		trigger interface{}
		pass    bool
		brokers []*eventing.Broker
	}{{
		name:    "unknown type",
		trigger: &eventing.Broker{},
		pass:    false,
	}, {
		name: "non matching broker",
		trigger: &eventing.Trigger{
			Spec: eventing.TriggerSpec{
				Broker: "does-not-exists",
			},
		},
		pass: false,
	}, {
		name: "exiting matching broker",
		trigger: &eventing.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "tr",
			},
			Spec: eventing.TriggerSpec{
				Broker: "br",
			},
		},
		brokers: []*eventing.Broker{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "br",
				Annotations: map[string]string{
					eventing.BrokerClassAnnotationKey: apiseventing.MTChannelBrokerClassValue,
				},
			},
		}},
		pass: true,
	}, {
		name: "exiting non matching broker",
		trigger: &eventing.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "tr",
			},
			Spec: eventing.TriggerSpec{
				Broker: "br",
			},
		},
		brokers: []*eventing.Broker{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "br",
				Annotations: map[string]string{
					eventing.BrokerClassAnnotationKey: "some-other-broker",
				},
			},
		}},
		pass: false,
	}}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			brokerInformer := brokerinformer.Get(ctx)
			for _, obj := range tc.brokers {
				_ = brokerInformer.Informer().GetStore().Add(obj)
			}
			featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"))
			filter := filterTriggers(featureStore, brokerInformer.Lister())
			pass := filter(tc.trigger)
			assert.Equal(t, tc.pass, pass)
		})
	}
}

func TestGetTriggersForBroker(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []runtime.Object
		out  []string
	}{{
		name: "Empty",
	}, {
		name: "single matching",
		in:   []runtime.Object{testingv1.NewTrigger("match", testNS, brokerName)},
		out:  []string{"match"},
	}, {
		name: "two, only one matching",
		in:   []runtime.Object{testingv1.NewTrigger("match", testNS, brokerName), testingv1.NewTrigger("nomatch", testNS, "anotherbroker")},
		out:  []string{"match"},
	}, {
		name: "two, both one matching",
		in:   []runtime.Object{testingv1.NewTrigger("match", testNS, brokerName), testingv1.NewTrigger("match2", testNS, brokerName)},
		out:  []string{"match", "match2"},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			ls := testingv1.NewListers(tt.in)
			logger := logtesting.TestLogger(t)
			triggerLister := ls.GetTriggerLister()
			triggers := getTriggersForBroker(logger, triggerLister, ReadyBroker())
			var found []string
			for _, want := range tt.out {
				for _, got := range triggers {
					if got.Name == want {
						found = append(found, got.Name)
					}
				}
			}
			if len(found) != len(tt.out) {
				t.Fatalf("Did not find all the triggers, wanted %+v found %+v", tt.out, found)
			}
		})
	}
}

type TriggerListerFailer struct{}

func (failer *TriggerListerFailer) List(selector labels.Selector) (ret []*eventing.Trigger, err error) {
	return nil, nil
}

func (failer *TriggerListerFailer) Triggers(namespace string) v1lister.TriggerNamespaceLister {
	return &TriggerNamespaceListerFailer{}
}

type TriggerNamespaceListerFailer struct{}

// List lists all Triggers in the indexer.
// Objects returned here must be treated as read-only.
func (failer *TriggerNamespaceListerFailer) List(selector labels.Selector) (ret []*eventing.Trigger, err error) {
	return nil, fmt.Errorf("Inducing test failure for List")
}

// Triggers returns an object that can list and get Triggers.
func (failer *TriggerNamespaceListerFailer) Get(name string) (*eventing.Trigger, error) {
	return nil, nil
}

func TestListFailure(t *testing.T) {
	logger := logtesting.TestLogger(t)
	triggerListerFailer := &TriggerListerFailer{}
	if len(getTriggersForBroker(logger, triggerListerFailer, ReadyBroker())) != 0 {
		t.Fatalf("Got back triggers when not expecting any")
	}
}
