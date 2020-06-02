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

package duck

import (
	"context"
	"crypto/md5"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/reconciler/source/duck/resources"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/source"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"

	. "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS     = "test-namespace"
	sourceName = "test-source"
	sourceUID  = "test-source-uid"
	sinkName   = "testsink"

	crdName = "testsources.testing.sources.knative.dev"
)

var (
	brokerDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1beta1",
		},
	}

	gvr = schema.GroupVersionResource{
		Group:    "testing.sources.knative.dev",
		Version:  "v1alpha1",
		Resource: "testsources",
	}

	gvk = schema.GroupVersionKind{
		Group:   "testing.sources.knative.dev",
		Version: "v1alpha1",
		Kind:    "TestSource",
	}
)

func TestAllCases(t *testing.T) {
	// key := testNS + "/" + sourceName
	table :=
		TableTest{{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "valid source with broker sink, create event types",
			Objects: []runtime.Object{
				makeSource([]duckv1.CloudEventAttributes{{
					Type:   "my-type-1",
					Source: "http://my-source-1",
				}, {
					Type:   "my-type-2",
					Source: "http://my-source-1",
				}}),
			},
			Key: testNS + "/" + sourceName,
			WantCreates: []runtime.Object{
				makeEventType("my-type-1", "http://my-source-1"),
				makeEventType("my-type-2", "http://my-source-1"),
			},
		}, {
			Name: "valid source with broker sink, delete event type",
			Objects: []runtime.Object{
				makeSource([]duckv1.CloudEventAttributes{{
					Type:   "my-type-1",
					Source: "http://my-source-1",
				}, {
					Type:   "my-type-2",
					Source: "http://my-source-1",
				}}),
				// https://github.com/knative/pkg/issues/411
				// Be careful adding more EventTypes here, the current unit test lister does not
				// return items in a fixed order, so the EventTypes can come back in any order.
				// WantDeletes requires the order to be correct, so will be flaky if we add more
				// than one EventType here.
				makeEventTypeWithName("other-type", "my-source-1", "name-1"),
			},
			Key: testNS + "/" + sourceName,
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{Name: "name-1"},
			},
			WantCreates: []runtime.Object{
				makeEventType("my-type-1", "http://my-source-1"),
				makeEventType("my-type-2", "http://my-source-1"),
			},
		}, {
			Name: "valid source with broker sink, create missing event types",
			Objects: []runtime.Object{
				makeSource([]duckv1.CloudEventAttributes{{
					Type:   "my-type-1",
					Source: "http://my-source-1",
				}, {
					Type:   "my-type-2",
					Source: "http://my-source-1",
				}}),
				makeEventType("my-type-1", "http://my-source-1"),
			},
			Key: testNS + "/" + sourceName,
			WantCreates: []runtime.Object{
				makeEventType("my-type-2", "http://my-source-1"),
			},
			// TODO add tests that read the CRD registry annotation.
		}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = source.WithDuck(ctx)
		_, sourceLister, _ := source.Get(ctx).Get(gvr)
		return &Reconciler{
			crdLister:         listers.GetCustomResourceDefinitionLister(),
			eventTypeLister:   listers.GetEventTypeLister(),
			sourceLister:      sourceLister,
			gvr:               gvr,
			crdName:           crdName,
			eventingClientSet: fakeeventingclient.Get(ctx),
		}
	},
		false, logger,
	))
}

func makeSource(attributes []duckv1.CloudEventAttributes) *duckv1.Source {
	return &duckv1.Source{
		TypeMeta: metav1.TypeMeta{
			Kind:       gvk.Kind,
			APIVersion: gvk.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceName,
			Namespace: testNS,
			UID:       sourceUID,
		},
		Spec: duckv1.SourceSpec{
			Sink: brokerDest,
		},
		Status: duckv1.SourceStatus{
			CloudEventAttributes: attributes,
		},
	}
}

func makeEventType(ceType, ceSource string) *v1beta1.EventType {
	ceSourceURL, _ := apis.ParseURL(ceSource)
	return &v1beta1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%x", md5.Sum([]byte(ceType+ceSource+sourceUID))),
			Labels:    resources.Labels(sourceName),
			Namespace: testNS,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         gvk.GroupVersion().String(),
				Kind:               gvk.Kind,
				Name:               sourceName,
				UID:                sourceUID,
				BlockOwnerDeletion: ptr.Bool(true),
				Controller:         ptr.Bool(true),
			}},
		},
		Spec: v1beta1.EventTypeSpec{
			Type:   ceType,
			Source: ceSourceURL,
			Broker: sinkName,
		},
	}
}

func makeEventTypeWithName(ceType, ceSource, name string) *v1beta1.EventType {
	et := makeEventType(ceType, ceSource)
	et.Name = name
	return et
}
