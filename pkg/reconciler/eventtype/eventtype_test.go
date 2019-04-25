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

package eventtype

import (
	"fmt"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	. "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	testNS          = "test-namespace"
	eventTypeName   = "test-eventtype"
	eventTypeType   = "test-type"
	eventTypeBroker = "test-broker"
	eventTypeSource = "/test-source"
)

var (
	trueVal = true

	testKey = fmt.Sprintf("%s/%s", testNS, eventTypeName)
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		},
		{
			Name: "EventType not found",
			Key:  testKey,
		},
		{
			Name: "EventType being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithInitEventTypeConditions,
					WithEventTypeDeletionTimestamp),
			},
		},
		{
			Name: "Broker not found",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
					WithInitEventTypeConditions,
					WithEventTypeBrokerDoesNotExist,
				),
			}},
		},
		{
			Name: "Broker not ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
				),
				NewBroker(eventTypeBroker, testNS,
					WithInitBrokerConditions,
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
					WithEventTypeBrokerExists,
					WithEventTypeBrokerNotReady,
				),
			}},
		},
		{
			Name: "Successful reconcile, became ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
				),
				NewBroker(eventTypeBroker, testNS,
					WithBrokerReady,
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewEventType(eventTypeName, testNS,
					WithEventTypeType(eventTypeType),
					WithEventTypeSource(eventTypeSource),
					WithEventTypeBroker(eventTypeBroker),
					WithEventTypeBrokerExists,
					WithEventTypeBrokerReady,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, eventTypeReadinessChanged, "EventType %q became ready", eventTypeName),
			},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:            reconciler.NewBase(opt, controllerAgentName),
			eventTypeLister: listers.GetEventTypeLister(),
			brokerLister:    listers.GetBrokerLister(),
		}
	}))

}

//func makeEventType() *v1alpha1.EventType {
//	return &v1alpha1.EventType{
//		TypeMeta: metav1.TypeMeta{
//			APIVersion: "eventing.knative.dev/v1alpha1",
//			Kind:       "EventType",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace: testNS,
//			Name:      eventTypeName,
//		},
//		Spec: v1alpha1.EventTypeSpec{
//			Broker: eventTypeBroker,
//			Type:   eventTypeType,
//		},
//	}
//}

//func makeReadyEventType() *v1alpha1.EventType {
//	t := makeEventType()
//	t.Status.InitializeConditions()
//	t.Status.MarkBrokerExists()
//	t.Status.MarkBrokerReady()
//	return t
//}
//
//func makeDeletingEventType() *v1alpha1.EventType {
//	et := makeReadyEventType()
//	et.DeletionTimestamp = &deletionTime
//	return et
//}
//
//func makeBroker() *v1alpha1.Broker {
//	return &v1alpha1.Broker{
//		TypeMeta: metav1.TypeMeta{
//			APIVersion: "eventing.knative.dev/v1alpha1",
//			Kind:       "Broker",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace: testNS,
//			Name:      eventTypeBroker,
//		},
//		Spec: v1alpha1.BrokerSpec{
//			ChannelTemplate: &v1alpha1.ChannelSpec{
//				Provisioner: makeChannelProvisioner(),
//			},
//		},
//	}
//}
//
//func makeBrokerReady() *v1alpha1.Broker {
//	b := makeBroker()
//	b.Status.InitializeConditions()
//	b.Status.MarkTriggerChannelReady()
//	b.Status.MarkFilterReady()
//	b.Status.MarkIngressReady()
//	b.Status.SetAddress("test-address")
//	b.Status.MarkIngressChannelReady()
//	b.Status.MarkIngressSubscriptionReady()
//	return b
//}
//
//func makeChannelProvisioner() *corev1.ObjectReference {
//	return &corev1.ObjectReference{
//		APIVersion: "eventing.knative.dev/v1alpha1",
//		Kind:       "ClusterChannelProvisioner",
//		Name:       "my-provisioner",
//	}
//}
