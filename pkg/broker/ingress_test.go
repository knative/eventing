/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package broker

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.uber.org/zap"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	broker     = "test-broker"
	namespace  = "test-namespace"
	testType   = "test-type"
	otherType  = "other-test-type"
	testSource = "/test-source"
	testFrom   = "/test-from"
)

func init() {
	// Add types to scheme.
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestIngress(t *testing.T) {

	extensions := map[string]interface{}{
		extensionFrom: testFrom,
	}

	testCases := map[string]struct {
		eventTypes []*eventingv1alpha1.EventType
		mocks      controllertesting.Mocks
		event      cloudevents.Event
		policySpec *eventingv1alpha1.IngressPolicySpec
		want       bool
		// TODO add wantPresent to check creation of an EventType.
	}{
		"allow any, accept": {
			policySpec: &eventingv1alpha1.IngressPolicySpec{
				AllowAny: true,
			},
			want: true,
		},
		"allow registered, error listing types, reject": {
			policySpec: &eventingv1alpha1.IngressPolicySpec{
				AllowAny: false,
			},
			event: makeCloudEvent(nil),
			mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("error listing types")
					},
				},
			},
			want: false,
		},
		"allow registered, event not found, reject": {
			policySpec: &eventingv1alpha1.IngressPolicySpec{
				AllowAny: false,
			},
			event: makeCloudEvent(nil),
			eventTypes: []*eventingv1alpha1.EventType{
				makeEventType(otherType, testSource),
			},
			want: false,
		},
		"allow registered, event not found with from, reject": {
			policySpec: &eventingv1alpha1.IngressPolicySpec{
				AllowAny: false,
			},
			event: makeCloudEvent(extensions),
			eventTypes: []*eventingv1alpha1.EventType{
				makeEventType(testType, testSource),
			},
			want: false,
		},
		"allow registered, event registered, accept": {
			policySpec: &eventingv1alpha1.IngressPolicySpec{
				AllowAny: false,
			},
			event: makeCloudEvent(nil),
			eventTypes: []*eventingv1alpha1.EventType{
				makeEventType(testType, testSource),
			},
			want: true,
		},
		"allow registered, event registered with from, accept": {
			policySpec: &eventingv1alpha1.IngressPolicySpec{
				AllowAny: false,
			},
			event: makeCloudEvent(extensions),
			eventTypes: []*eventingv1alpha1.EventType{
				makeEventType(testType, testFrom),
			},
			want: true,
		},
		"auto add, error listing types, accept": {
			policySpec: &eventingv1alpha1.IngressPolicySpec{
				AutoAdd: true,
			},
			event: makeCloudEvent(nil),
			mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("error listing types")
					},
				},
			},
			want: true,
		},
		"auto add, error creating type, accept": {
			policySpec: &eventingv1alpha1.IngressPolicySpec{
				AutoAdd: true,
			},
			event: makeCloudEvent(nil),
			mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("error creating type")
					},
				},
			},
			want: true,
		},
		"auto add, created type, accept": {
			policySpec: &eventingv1alpha1.IngressPolicySpec{
				AutoAdd: true,
			},
			event: makeCloudEvent(nil),
			mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						et := obj.(*eventingv1alpha1.EventType)
						expected := makeEventType(testType, testSource).Spec
						if !equality.Semantic.DeepDerivative(et.Spec, expected) {
							return controllertesting.Handled, errors.New("error creating type")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			want: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {

			ctx := context.TODO()
			objs := make([]runtime.Object, 0, len(tc.eventTypes))
			for _, et := range tc.eventTypes {
				objs = append(objs, et)
			}

			c := newClient(objs, tc.mocks)
			policy := NewPolicy(zap.NewNop(), c, tc.policySpec, namespace, broker, false)

			got := policy.AllowEvent(ctx, tc.event)

			if tc.want != got {
				t.Errorf("want %t, got %t", tc.want, got)
			}

		})
	}
}

func newClient(initial []runtime.Object, mocks controllertesting.Mocks) *controllertesting.MockClient {
	innerClient := fake.NewFakeClient(initial...)
	return controllertesting.NewMockClient(innerClient, mocks)
}

func makeCloudEvent(extensions map[string]interface{}) cloudevents.Event {
	return cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type: testType,
			Source: types.URLRef{
				URL: url.URL{
					Path: testSource,
				},
			},
			ContentType: cloudevents.StringOfApplicationJSON(),
			Extensions:  extensions,
		},
	}
}

func makeEventType(eventType, eventSource string) *eventingv1alpha1.EventType {
	return &eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", eventType),
			Namespace:    namespace,
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:   eventType,
			Broker: broker,
			Source: eventSource,
		},
	}
}
