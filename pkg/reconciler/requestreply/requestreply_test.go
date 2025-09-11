/*
Copyright 2025 The Knative Authors

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

package requestreply

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	addressablev1 "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	reconcilertesting "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/system"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	requestreplyreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/requestreply"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	. "knative.dev/eventing/pkg/reconciler/testing/v1alpha1"
)

const (
	testNamespace    = "test-namespace"
	requestReplyName = "test-requestReply"
	brokerName       = "test-broker"
)

var (
	testKey   = fmt.Sprintf("%s/%s", testNamespace, requestReplyName)
	brokerRef = duckv1.KReference{
		Kind:       "Broker",
		APIVersion: "eventing.knative.dev/v1",
		Name:       brokerName,
	}

	requestReplyAddress = &duckv1.Addressable{
		Name: ptr.To("http"),
		URL: &apis.URL{
			Scheme: "http",
			Host:   network.GetServiceHostname("request-reply", system.Namespace()),
			Path:   fmt.Sprintf("/%s/%s", testNamespace, requestReplyName),
		},
	}

	ignoreSecretData = cmpopts.IgnoreFields(corev1.Secret{}, "Data") // needed as the secrets are rng
)

func TestReconcile(t *testing.T) {

	table := reconcilertesting.TableTest{
		{
			Name: "bad work queue key",
			Key:  "too/many/parts",
		},
		{
			Name: "key not found",
			Key:  "foo/not-found",
		},
		{
			Name: "RequestReply not found",
			Key:  testKey,
		},
		{
			Name: "Successful reconciliation",
			Key:  testKey,
			Objects: []runtime.Object{
				NewRequestReply(requestReplyName, testNamespace,
					WithRequestReplyBroker(brokerRef),
					WithInitRequestReplyConditions),
				NewBroker(brokerName, testNamespace,
					WithBrokerReady),
				NewStatefulSet("request-reply", system.Namespace(),
					WithStatefulSetReplicas(1)),
				NewPod("request-reply-0", system.Namespace(),
					WithPodIP("127.0.0.1"),
					WithPodReady()),
				NewTrigger(fmt.Sprintf("%s0-1", requestReplyName), testNamespace,
					brokerName,
					WithTriggerFilters(
						[]eventingv1.SubscriptionsAPIFilter{
							{
								CESQL: fmt.Sprintf("KN_VERIFY_CORRELATION_ID(%s, \"%s\", \"%s\", \"%s\", %d, %d)",
									"replyid",
									requestReplyName,
									testNamespace,
									"request-reply-keys",
									0,
									1,
								),
							},
						},
					),
					WithTriggerSubscriber(duckv1.Destination{
						URI: &apis.URL{
							Scheme: "http",
							Host:   "127.0.0.1:8080",
							Path:   fmt.Sprintf("/%s/%s/reply", testNamespace, requestReplyName),
						},
					}),
					WithLabels(map[string]string{
						"eventing.knative.dev/RequestReply.name":              requestReplyName,
						"eventing.knative.dev/RequestReply.namespace":         testNamespace,
						"eventing.knative.dev/RequestReply.dataPlaneReplicas": "1",
					}),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscribed(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				NewSecret("request-reply-keys", system.Namespace(),
					WithSecretData(map[string][]byte{
						fmt.Sprintf("%s.%s.key-0", testNamespace, requestReplyName): make([]byte, 32),
					})),
			},
			SkipNamespaceValidation: true, // needed as secrets are created in a different ns than the requestreply
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: NewRequestReply(requestReplyName, testNamespace,
						WithRequestReplyBroker(brokerRef),
						WithRequestReplyTriggersReady,
						WithRequestReplyBrokerReady,
						WithRequestReplyEventPoliciesReady,
						WithRequestReplyAddress(requestReplyAddress),
						WithRequestReplyReplicas(1, 1),
						WithRequestReplyBrokerAddressAnnotation("http://example.com")),
				},
			},
			CmpOpts: []cmp.Option{ignoreSecretData},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, l *Listers, w configmap.Watcher) controller.Reconciler {
		ctx = addressablev1.WithDuck(ctx)
		r := &Reconciler{
			kubeClient:        fakekubeclient.Get(ctx),
			eventingClient:    fakeeventingclient.Get(ctx),
			secretLister:      l.GetSecretLister(),
			triggerLister:     l.GetTriggerLister(),
			brokerLister:      l.GetBrokerLister(),
			statefulSetLister: l.GetStatefulSetLister(),
			deleteContext:     ctx,
		}

		return requestreplyreconciler.NewReconciler(
			ctx,
			logger,
			fakeeventingclient.Get(ctx),
			l.GetRequestReplyLister(),
			controller.GetEventRecorder(ctx),
			r,
		)
	},
		false,
		logger,
	))
}
