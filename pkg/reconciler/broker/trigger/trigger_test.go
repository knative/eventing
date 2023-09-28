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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1addr "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/source"
	v1a1addr "knative.dev/pkg/client/injection/ducks/duck/v1alpha1/addressable"
	v1b1addr "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/apis/sources/v1beta2"
	"knative.dev/eventing/pkg/auth"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/pkg/reconciler/broker/resources"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"

	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"

	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	rtv1beta2 "knative.dev/eventing/pkg/reconciler/testing/v1beta2"
)

const (
	systemNS   = "knative-testing"
	testNS     = "test-namespace"
	brokerName = "test-broker"
	dlsName    = "test-dls"

	configMapName = "test-configmap"

	triggerName = "test-trigger"
	triggerUID  = "test-trigger-uid"

	triggerChannelAPIVersion = "messaging.knative.dev/v1"
	triggerChannelKind       = "InMemoryChannel"
	triggerChannelName       = "test-broker-kne-trigger"

	subscriberURI           = "http://example.com/subscriber/"
	subscriberKind          = "Service"
	subscriberName          = "subscriber-name"
	subscriberNameNamespace = "subscriber-namespace"
	subscriberGroup         = "serving.knative.dev"
	subscriberVersion       = "v1"

	pingSourceName              = "test-ping-source"
	testSchedule                = "*/2 * * * *"
	testContentType             = cloudevents.TextPlain
	testData                    = "data"
	sinkName                    = "testsink"
	dependencyAnnotation        = `{"kind":"PingSource","name":"test-ping-source","apiVersion":"sources.knative.dev/v1beta2"}`
	subscriberURIReference      = "foo"
	subscriberResolvedTargetURI = "http://example.com/subscriber/foo"

	k8sServiceResolvedURI = "http://subscriber-name.test-namespace.svc.cluster.local"
	currentGeneration     = 1
	outdatedGeneration    = 0

	imcSpec = `
apiVersion: "messaging.knative.dev/v1"
kind: "InMemoryChannel"
`
)

var (
	subscriberURL, _ = apis.ParseURL(subscriberURI)

	testKey = fmt.Sprintf("%s/%s", testNS, triggerName)

	triggerChannelHostname = network.GetServiceHostname("foo", "bar")
	triggerChannelURL      = fmt.Sprintf("http://%s", triggerChannelHostname)

	filterServiceName  = "broker-filter"
	ingressServiceName = "broker-ingress"

	subscriptionName = fmt.Sprintf("%s-%s-%s", brokerName, triggerName, triggerUID)

	subscriberAPIVersion = fmt.Sprintf("%s/%s", subscriberGroup, subscriberVersion)
	subscriberGVK        = metav1.GroupVersionKind{
		Group:   subscriberGroup,
		Version: subscriberVersion,
		Kind:    subscriberKind,
	}
	k8sServiceGVK = metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}
	brokerDestv1 = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1",
		},
	}
	dlsSVCDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       dlsName,
			Kind:       "Service",
			APIVersion: "v1",
			Namespace:  testNS,
		},
	}
	k8sSVCDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       subscriberName,
			Kind:       "Service",
			APIVersion: "v1",
			Namespace:  testNS,
		},
	}
	k8sSVCDestAddr = duckv1.Addressable{
		URL: apis.HTTP(network.GetServiceHostname(subscriberName, testNS)),
	}

	sinkDNS = network.GetServiceHostname("sink", "mynamespace")
	sinkURI = "http://" + sinkDNS

	brokerAddress = &apis.URL{
		Scheme: "http",
		Host:   network.GetServiceHostname(ingressServiceName, systemNS),
		Path:   fmt.Sprintf("/%s/%s", testNS, brokerName),
	}

	brokerDLS = duckv1.Addressable{
		URL:     apis.HTTP("test-dls.test-namespace.svc.cluster.local"),
		CACerts: nil,
	}

	dlsURL, _ = apis.ParseURL("http://example.com")
)

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
		}, {
			Name: "Trigger not found",
			Key:  testKey,
		}, {
			Name:    "Trigger is being deleted",
			Key:     testKey,
			Objects: []runtime.Object{NewTrigger(triggerName, testNS, brokerName, WithTriggerDeleted)},
		}, {
			Name: "Broker does not exist",
			Key:  testKey,
			Objects: []runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "test-broker" does not exist`)),
			}},
		}, {
			Name: "Not my broker class - no status updates",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass("not-my-broker"),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				NewTrigger(triggerName, testNS, brokerName,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI)),
			},
		}, {
			Name: "Broker not reconciled yet",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config())),
				NewTrigger(triggerName, testNS, brokerName,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerNotConfigured()),
			},
		}, {
			Name: "Broker not ready yet",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithFilterFailed("nofilter", "NoFilter")),
				NewTrigger(triggerName, testNS, brokerName,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerFailed("nofilter", "NoFilter")),
			},
		}, {
			Name: "Creates subscription",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName)),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WantCreates: []runtime.Object{
				makeFilterSubscription(testNS),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
		}, {
			Name: "Creates subscription with retry from trigger",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName)),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerRetry(5, nil, nil)),
			},
			WantCreates: []runtime.Object{
				resources.NewSubscription(makeTrigger(testNS), createTriggerChannelRef(), makeBrokerRef(), makeServiceURI(), makeDelivery(nil, ptr.Int32(5), nil, nil)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerRetry(5, nil, nil),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
		}, {
			Name: "Creates subscription with dls from trigger",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName)),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerDeadLeaderSink(duckv1.Destination{URI: dlsURL})),
			},
			WantCreates: []runtime.Object{
				resources.NewSubscription(makeTrigger(testNS), createTriggerChannelRef(), makeBrokerRef(), makeServiceURI(), makeDelivery(&duckv1.Destination{URI: dlsURL}, nil, nil, nil)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerDeadLeaderSink(duckv1.Destination{URI: dlsURL}),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerStatusDeadLetterSinkURI(duckv1.Addressable{URL: dlsURL}),
					WithTriggerDeadLetterSinkResolvedSucceeded(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
		}, {
			Name: "TLS: Creates subscription with dls from trigger",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName)),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriber(duckv1.Destination{
						URI:     subscriberURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerDeadLeaderSink(duckv1.Destination{
						URI:     dlsURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					})),
			},
			WantCreates: []runtime.Object{
				resources.NewSubscription(
					makeTrigger(testNS),
					createTriggerChannelRef(),
					makeBrokerRef(),
					makeServiceURI(),
					makeDelivery(&duckv1.Destination{URI: dlsURL, CACerts: pointer.String(string(eventingtlstesting.CA))}, nil, nil, nil),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriber(duckv1.Destination{
						URI:     subscriberURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerDeadLeaderSink(duckv1.Destination{
						URI:     dlsURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerStatusSubscriberCACerts(string(eventingtlstesting.CA)),
					WithTriggerStatusDeadLetterSinkURI(duckv1.Addressable{URL: dlsURL}),
					WithTriggerDeadLetterSinkResolvedSucceeded(),
					WithTriggerDeadLetterSinkCACerts(string(eventingtlstesting.CA)),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "TLS: Creates subscription with TLS subscriber (permissive)",
			Key:  testKey,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.TransportEncryption: feature.Permissive,
			}),
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName)),
				NewSecret(eventingtls.BrokerFilterServerTLSSecretName, systemNS, WithSecretData(map[string][]byte{
					"ca.crt": eventingtlstesting.CA,
				})),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriber(duckv1.Destination{
						URI:     subscriberURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerDeadLeaderSink(duckv1.Destination{
						URI:     dlsURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					})),
			},
			WantCreates: []runtime.Object{
				resources.NewSubscription(
					makeTrigger(testNS),
					createTriggerChannelRef(),
					makeBrokerRef(),
					makeServiceURIHTTPS(),
					makeDelivery(&duckv1.Destination{URI: dlsURL, CACerts: pointer.String(string(eventingtlstesting.CA))}, nil, nil, nil),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriber(duckv1.Destination{
						URI:     subscriberURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerDeadLeaderSink(duckv1.Destination{
						URI:     dlsURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerStatusSubscriberCACerts(string(eventingtlstesting.CA)),
					WithTriggerStatusDeadLetterSinkURI(duckv1.Addressable{URL: dlsURL}),
					WithTriggerDeadLetterSinkResolvedSucceeded(),
					WithTriggerDeadLetterSinkCACerts(string(eventingtlstesting.CA)),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "TLS: Creates subscription with TLS subscriber (strict)",
			Key:  testKey,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.TransportEncryption: feature.Strict,
			}),
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName)),
				NewSecret(eventingtls.BrokerFilterServerTLSSecretName, systemNS, WithSecretData(map[string][]byte{
					"ca.crt": eventingtlstesting.CA,
				})),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriber(duckv1.Destination{
						URI:     subscriberURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerDeadLeaderSink(duckv1.Destination{
						URI:     dlsURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					})),
			},
			WantCreates: []runtime.Object{
				resources.NewSubscription(
					makeTrigger(testNS),
					createTriggerChannelRef(),
					makeBrokerRef(),
					makeServiceURIHTTPS(),
					makeDelivery(&duckv1.Destination{URI: dlsURL, CACerts: pointer.String(string(eventingtlstesting.CA))}, nil, nil, nil),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriber(duckv1.Destination{
						URI:     subscriberURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerDeadLeaderSink(duckv1.Destination{
						URI:     dlsURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerStatusSubscriberCACerts(string(eventingtlstesting.CA)),
					WithTriggerStatusDeadLetterSinkURI(duckv1.Addressable{URL: dlsURL}),
					WithTriggerDeadLetterSinkResolvedSucceeded(),
					WithTriggerDeadLetterSinkCACerts(string(eventingtlstesting.CA)),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "TLS: Creates subscription with dls from broker",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithDeadLeaderSink(duckv1.Destination{
						URI:     dlsURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithBrokerStatusDLS(duckv1.Addressable{
						URL:     dlsURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
				),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriber(duckv1.Destination{
						URI:     subscriberURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
				),
			},
			WantCreates: []runtime.Object{
				resources.NewSubscription(
					makeTrigger(testNS),
					createTriggerChannelRef(),
					makeBrokerRef(),
					makeServiceURI(),
					makeDelivery(&duckv1.Destination{URI: dlsURL, CACerts: pointer.String(string(eventingtlstesting.CA))}, nil, nil, nil),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriber(duckv1.Destination{
						URI:     subscriberURL,
						CACerts: pointer.String(string(eventingtlstesting.CA)),
					}),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerStatusSubscriberCACerts(string(eventingtlstesting.CA)),
					WithTriggerStatusDeadLetterSinkURI(duckv1.Addressable{URL: dlsURL}),
					WithTriggerDeadLetterSinkResolvedSucceeded(),
					WithTriggerDeadLetterSinkCACerts(string(eventingtlstesting.CA)),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "Subscription Create fails",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedSucceeded()),
			},
			WantCreates: []runtime.Object{
				makeFilterSubscription(testNS),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions"),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create subscriptions"),
			},
			WantErr: true,
		}, {
			Name: "Trigger subscription create fails, update status fails",
			Key:  testKey,
			Objects: []runtime.Object{
				ReadyBroker(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
				InduceFailure("update", "triggers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions"),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(testNS),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-trigger": inducing failure for update triggers`),
			},
			WantErr: true,
		}, {
			Name: "Trigger subscription not owned by Trigger",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeFilterSubscriptionNotOwnedByTrigger()}...),
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerUID(triggerUID),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerNotSubscribed("NotSubscribed", `trigger "test-trigger" does not own subscription "test-broker-test-trigger-test-trigger-uid"`),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `trigger "test-trigger" does not own subscription "test-broker-test-trigger-test-trigger-uid"`),
			},
			WantErr: true,
		}, {
			Name: "Trigger subscription update works",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeDifferentReadySubscription()}...),
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyReady(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  eventingduckv1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Name: subscriptionName,
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(testNS),
			},
		}, {
			Name: "Trigger subscription update (delete) fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeDifferentReadySubscription()}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("delete", "subscriptions"),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for delete subscriptions"),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  eventingduckv1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Name: subscriptionName,
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionDeleteFailed", `Delete Trigger's subscription failed: inducing failure for delete subscriptions`),
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for delete subscriptions"),
			},
		}, {
			Name: "Trigger subscription create after delete fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeDifferentReadySubscription()}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions"),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", `Create Trigger's subscription failed: inducing failure for create subscriptions`),
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create subscriptions"),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  eventingduckv1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Name: subscriptionName,
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(testNS),
			},
		}, {
			Name: "Trigger has subscriber ref exists",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberAddressableAsUnstructured(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitTriggerConditions)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyReady(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(testNS),
			},
		}, {
			Name: "Trigger has subscriber ref exists and URI",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberAddressableAsUnstructured(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, testNS, subscriberURIReference),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, testNS, subscriberURIReference),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberResolvedTargetURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyReady(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(testNS),
			},
		}, {
			Name: "Trigger has subscriber ref exists kubernetes Service",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberKubernetesServiceAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(k8sServiceGVK, subscriberName, testNS),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(k8sServiceGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(k8sServiceResolvedURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyReady(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(testNS),
			},
		}, {
			Name: "Trigger has subscriber ref doesn't exist",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitTriggerConditions,
				)}...),
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `failed to get object test-namespace/subscriber-name: services.serving.knative.dev "subscriber-name" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedFailed("Unable to get the Subscriber's URI", `failed to get object test-namespace/subscriber-name: services.serving.knative.dev "subscriber-name" not found`),
				),
			}},
			WantErr: true,
		}, {
			Name: "Trigger has a dls ref that doesn't exist",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberAddressableAsUnstructured(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerDeadLeaderSink(brokerDestv1),
				)}...),
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `failed to get object test-namespace/testsink: brokers.eventing.knative.dev "testsink" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerDeadLeaderSink(brokerDestv1),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkResolvedFailed("Unable to get the dead letter sink's URI", `failed to get object test-namespace/testsink: brokers.eventing.knative.dev "testsink" not found`),
				),
			}},
			WantErr: true,
		}, {
			Name: "Trigger has a valid dls ref and goes ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberKubernetesServiceAsUnstructured(),
				makeDLSServiceAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(k8sServiceGVK, subscriberName, testNS),
					WithInitTriggerConditions,
					WithTriggerDeadLeaderSink(dlsSVCDest),
				)}...),
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(k8sServiceGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerDeadLeaderSink(dlsSVCDest),
					WithTriggerDependencyReady(),
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(k8sServiceResolvedURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusDeadLetterSinkURI(brokerDLS),
					WithTriggerDeadLetterSinkResolvedSucceeded(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantCreates: []runtime.Object{
				resources.NewSubscription(
					makeTrigger(testNS),
					createTriggerChannelRef(),
					makeBrokerRef(),
					makeServiceURI(),
					makeDelivery(&dlsSVCDest, nil, nil, nil),
				),
			},
			WantErr: false,
		}, {
			Name: "Broker has a dls ref that doesn't exist",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithDeadLeaderSink(brokerDestv1),
					WithDLSResolvedFailed(),
				),
				makeSubscriberAddressableAsUnstructured(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerFailed("Unable to get the DeadLetterSink's URI", `brokers.eventing.knative.dev "testsink" not found`),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
				),
			}},
		}, {
			Name: "Trigger has no dls ref, Broker has a valid dls ref, Trigger fallbacks to it and goes ready",
			Key:  testKey,
			Objects: []runtime.Object{
				makeDLSServiceAsUnstructured(),
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithBrokerResourceVersion(""),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithDeadLeaderSink(dlsSVCDest),
					WithBrokerStatusDLS(brokerDLS),
				),
				createChannel(testNS, true),
				imcConfigMap(),
				makeReadySubscription(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				resources.NewSubscription(
					makeTrigger(testNS),
					createTriggerChannelRef(),
					makeBrokerRef(),
					makeServiceURI(),
					makeDelivery(&dlsSVCDest, nil, nil, nil),
				),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  eventingduckv1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Name: subscriptionName,
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerDependencyReady(),
					WithTriggerSubscribed(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusDeadLetterSinkURI(brokerDLS),
					WithTriggerDeadLetterSinkResolvedSucceeded(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			// Same as previous test but using the legacy channel template configmap element.
			Name: "Trigger has no dls ref, Broker has a valid dls ref, Trigger fallbacks to it and goes ready. Using legacy channel template config element.",
			Key:  testKey,
			Objects: []runtime.Object{
				makeDLSServiceAsUnstructured(),
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithBrokerResourceVersion(""),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithDeadLeaderSink(dlsSVCDest),
					WithBrokerStatusDLS(brokerDLS),
				),
				createChannel(testNS, true),
				// Use the legacy channel template configmap element at this test.
				imcConfigMapLegacy(),
				makeReadySubscription(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				resources.NewSubscription(
					makeTrigger(testNS),
					createTriggerChannelRef(),
					makeBrokerRef(),
					makeServiceURI(),
					makeDelivery(&dlsSVCDest, nil, nil, nil),
				),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  eventingduckv1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Name: subscriptionName,
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerDependencyReady(),
					WithTriggerSubscribed(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusDeadLetterSinkURI(brokerDLS),
					WithTriggerDeadLetterSinkResolvedSucceeded(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "Trigger has a valid dls ref, Broker has a valid dls ref, Trigger uses it's dls and goes ready",
			Key:  testKey,
			Objects: []runtime.Object{
				makeDLSServiceAsUnstructured(),
				makeSubscriberKubernetesServiceAsUnstructured(),
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithBrokerResourceVersion(""),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithDeadLeaderSink(k8sSVCDest),
					WithBrokerStatusDLS(k8sSVCDestAddr),
				),
				createChannel(testNS, true),
				imcConfigMap(),
				makeReadySubscription(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerDeadLeaderSink(dlsSVCDest),
				),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				resources.NewSubscription(
					makeTrigger(testNS),
					createTriggerChannelRef(),
					makeBrokerRef(),
					makeServiceURI(),
					makeDelivery(&dlsSVCDest, nil, nil, nil),
				),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Resource:  eventingduckv1.SchemeGroupVersion.WithResource("subscriptions"),
				},
				Name: subscriptionName,
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerDependencyReady(),
					WithTriggerSubscribed(),
					WithTriggerDeadLeaderSink(dlsSVCDest),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusDeadLetterSinkURI(brokerDLS),
					WithTriggerDeadLetterSinkResolvedSucceeded(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "Trigger does not have DLS defined, Broker DLS URI resolved to nil, Trigger returns error status",
			Key:  testKey,
			Objects: []runtime.Object{
				makeSubscriberKubernetesServiceAsUnstructured(),
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerReady,
					WithBrokerResourceVersion(""),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithDeadLeaderSink(k8sSVCDest),
				),
				createChannel(testNS, true),
				imcConfigMap(),
				makeReadySubscription(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `broker test-broker didn't set status.deadLetterSinkURI`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerSubscribedUnknown("", ""),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkResolvedFailed("Broker test-broker didn't set status.deadLetterSinkURI", ""),
				),
			}},
		}, {
			Name: "Subscription not ready, trigger marked not ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeFalseStatusSubscription(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerNotSubscribed("testInducedError", "test induced error"),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyReady(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "Subscription ready, trigger marked ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerDependencyReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "Dependency doesn't exist",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `propagating dependency readiness: getting the dependency: pingsources.sources.knative.dev "test-ping-source" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyFailed("DependencyDoesNotExist", `Dependency does not exist: pingsources.sources.knative.dev "test-ping-source" not found`),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantErr: true,
		}, {
			Name: "The status of Dependency is False",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(testNS),
				makeFalseStatusPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyFailed("NotFound", ""),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "The status of Dependency is Unknown",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(testNS),
				makeUnknownStatusCronJobSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyUnknown("", ""),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		},
		{
			Name: "Dependency generation not equal",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(testNS),
				makeGenerationNotEqualPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyUnknown("GenerationNotEqual", fmt.Sprintf("The dependency's metadata.generation, %q, is not equal to its status.observedGeneration, %q.", currentGeneration, outdatedGeneration)),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled()),
			}},
		},
		{
			Name: "Dependency ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(testNS),
				makeReadyPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyReady(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		},
		{
			Name: "Subscriber Not Specific Namespace",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberAddressableAsUnstructured(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, "", subscriberURIReference),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, testNS, subscriberURIReference),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberResolvedTargetURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyReady(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(testNS),
			},
		},
		{
			Name: "Subscriber Specific Namespace",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberAddressableAsUnstructured(subscriberNameNamespace),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, subscriberNameNamespace, subscriberURIReference),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, subscriberNameNamespace, subscriberURIReference),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberResolvedTargetURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerDependencyReady(),
					WithTriggerOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantCreates: []runtime.Object{
				makeFilterSubscription(subscriberNameNamespace),
			},
		},
		{
			Name: "OIDC: creates OIDC service account",
			Key:  testKey,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(testNS),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerDependencyReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerOIDCIdentityCreatedSucceeded(),
					WithTriggerOIDCServiceAccountName(makeTriggerOIDCServiceAccount().Name),
				),
			}},
			WantCreates: []runtime.Object{
				makeTriggerOIDCServiceAccount(),
			},
		},
		{
			Name: "OIDC: Trigger not ready on invalid OIDC service account",
			Key:  testKey,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(testNS),
				makeTriggerOIDCServiceAccountWithoutOwnerRef(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
				)}...),
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerDependencyUnknown("", ""),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDeadLetterSinkNotConfigured(),
					WithTriggerOIDCIdentityCreatedFailed("Unable to resolve service account for OIDC authentication", fmt.Sprintf("service account %s not owned by Trigger %s", makeTriggerOIDCServiceAccountWithoutOwnerRef().Name, triggerName)),
					WithTriggerOIDCServiceAccountName(makeTriggerOIDCServiceAccountWithoutOwnerRef().Name),
					WithTriggerSubscribedUnknown("", ""),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", fmt.Sprintf("service account %s not owned by Trigger %s", makeTriggerOIDCServiceAccountWithoutOwnerRef().Name, triggerName)),
			},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)
		ctx = v1addr.WithDuck(ctx)
		ctx = source.WithDuck(ctx)
		r := &Reconciler{
			eventingClientSet:    fakeeventingclient.Get(ctx),
			dynamicClientSet:     fakedynamicclient.Get(ctx),
			kubeclient:           fakekubeclient.Get(ctx),
			subscriptionLister:   listers.GetSubscriptionLister(),
			triggerLister:        listers.GetTriggerLister(),
			secretLister:         listers.GetSecretLister(),
			serviceAccountLister: listers.GetServiceAccountLister(),

			brokerLister:    listers.GetBrokerLister(),
			configmapLister: listers.GetConfigMapLister(),
			sourceTracker:   duck.NewListableTrackerFromTracker(ctx, source.Get, tracker.New(func(types.NamespacedName) {}, 0)),
			uriResolver:     resolver.NewURIResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0)),
		}
		return trigger.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetTriggerLister(),
			controller.GetEventRecorder(ctx),
			r)

	},
		false,
		logger,
	))
}

func config() *duckv1.KReference {
	return &duckv1.KReference{
		Name:       configMapName,
		Namespace:  testNS,
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}
}

func imcConfigMap() *corev1.ConfigMap {
	return NewConfigMap(configMapName, testNS,
		WithConfigMapData(map[string]string{"channel-template-spec": imcSpec}))
}

// imcConfigMapLegacy returns a confirmap using the legacy configuration
// element channelTemplateSpec . This element name will be deprecated in
// favor of channel-template-spec.
func imcConfigMapLegacy() *corev1.ConfigMap {
	return NewConfigMap(configMapName, testNS,
		WithConfigMapData(map[string]string{"channelTemplateSpec": imcSpec}))
}

func createChannel(namespace string, ready bool) *unstructured.Unstructured {
	name := fmt.Sprintf("%s-kne-trigger", brokerName)
	labels := map[string]interface{}{
		eventing.BrokerLabelKey:                 brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
	annotations := map[string]interface{}{
		"eventing.knative.dev/scope": "cluster",
	}
	if ready {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "messaging.knative.dev/v1",
				"kind":       "InMemoryChannel",
				"metadata": map[string]interface{}{
					"creationTimestamp": nil,
					"namespace":         namespace,
					"name":              name,
					"ownerReferences": []interface{}{
						map[string]interface{}{
							"apiVersion":         "eventing.knative.dev/v1",
							"blockOwnerDeletion": true,
							"controller":         true,
							"kind":               "Broker",
							"name":               brokerName,
							"uid":                "",
						},
					},
					"labels":      labels,
					"annotations": annotations,
				},
				"status": map[string]interface{}{
					"address": map[string]interface{}{
						"url": triggerChannelURL,
					},
				},
			},
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "eventing.knative.dev/v1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Broker",
						"name":               brokerName,
						"uid":                "",
					},
				},
				"labels":      labels,
				"annotations": annotations,
			},
		},
	}
}

func createTriggerChannelRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "messaging.knative.dev/v1",
		Kind:       "InMemoryChannel",
		Namespace:  testNS,
		Name:       fmt.Sprintf("%s-kne-trigger", brokerName),
	}
}

func makeServiceURI() *duckv1.Destination {
	return &duckv1.Destination{
		URI: &apis.URL{
			Scheme: "http",
			Host:   network.GetServiceHostname("broker-filter", systemNS),
			Path:   fmt.Sprintf("/triggers/%s/%s/%s", testNS, triggerName, triggerUID),
		},
	}
}

func makeServiceURIHTTPS() *duckv1.Destination {
	return &duckv1.Destination{
		URI: &apis.URL{
			Scheme: "https",
			Host:   network.GetServiceHostname("broker-filter", systemNS),
			Path:   fmt.Sprintf("/triggers/%s/%s/%s", testNS, triggerName, triggerUID),
		},
		CACerts: pointer.String(string(eventingtlstesting.CA)),
	}
}

func makeFilterSubscription(subscriberNamespace string) *messagingv1.Subscription {
	return resources.NewSubscription(makeTrigger(subscriberNamespace), createTriggerChannelRef(), makeBrokerRef(), makeServiceURI(), makeEmptyDelivery())
}

func makeTrigger(subscriberNamespace string) *eventingv1.Trigger {
	return &eventingv1.Trigger{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1",
			Kind:       "Trigger",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      triggerName,
			UID:       triggerUID,
		},
		Spec: eventingv1.TriggerSpec{
			Broker: brokerName,
			Filter: &eventingv1.TriggerFilter{
				Attributes: map[string]string{"Source": "Any", "Type": "Any"},
			},
			Subscriber: duckv1.Destination{
				Ref: &duckv1.KReference{
					Name:       subscriberName,
					Namespace:  subscriberNamespace,
					Kind:       subscriberKind,
					APIVersion: subscriberAPIVersion,
				},
			},
		},
	}
}

func makeBrokerRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1",
		Kind:       "Broker",
		Namespace:  testNS,
		Name:       brokerName,
	}
}

func makeEmptyDelivery() *eventingduckv1.DeliverySpec {
	return nil
}

func makeDelivery(dls *duckv1.Destination, retry *int32, backoffPolicy *eventingduckv1.BackoffPolicyType, backoffDelay *string) *eventingduckv1.DeliverySpec {
	ds := &eventingduckv1.DeliverySpec{
		Retry:          retry,
		BackoffPolicy:  backoffPolicy,
		BackoffDelay:   backoffDelay,
		DeadLetterSink: dls,
	}
	return ds
}

func allBrokerObjectsReadyPlus(objs ...runtime.Object) []runtime.Object {
	brokerObjs := []runtime.Object{
		NewBroker(brokerName, testNS,
			WithBrokerClass(eventing.MTChannelBrokerClassValue),
			WithBrokerConfig(config()),
			WithInitBrokerConditions,
			WithBrokerReady,
			WithBrokerResourceVersion(""),
			WithBrokerAddressURI(brokerAddress),
			WithChannelAddressAnnotation(triggerChannelURL),
			WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
			WithChannelKindAnnotation(triggerChannelKind),
			WithChannelNameAnnotation(triggerChannelName)),
		createChannel(testNS, true),
		imcConfigMap(),
		NewEndpoints(filterServiceName, systemNS,
			WithEndpointsLabels(FilterLabels()),
			WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
		NewEndpoints(ingressServiceName, systemNS,
			WithEndpointsLabels(IngressLabels()),
			WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
	}
	return append(brokerObjs[:], objs...)
}

// Just so we can test subscription updates
func makeDifferentReadySubscription() *messagingv1.Subscription {
	s := makeFilterSubscription(testNS)
	s.Spec.Subscriber.URI = apis.HTTP("different.example.com")
	s.Status = *eventingv1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeFilterSubscriptionNotOwnedByTrigger() *messagingv1.Subscription {
	sub := makeFilterSubscription(testNS)
	sub.OwnerReferences = []metav1.OwnerReference{}
	return sub
}

func makeReadySubscription(subscriberNamespace string) *messagingv1.Subscription {
	s := makeFilterSubscription(subscriberNamespace)
	s.Status = *eventingv1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeSubscriberAddressableAsUnstructured(subscriberNamespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": subscriberAPIVersion,
			"kind":       subscriberKind,
			"metadata": map[string]interface{}{
				"namespace": subscriberNamespace,
				"name":      subscriberName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": subscriberURI,
				},
			},
		},
	}
}

func makeFalseStatusSubscription() *messagingv1.Subscription {
	s := makeFilterSubscription(testNS)
	s.Status.MarkReferencesNotResolved("testInducedError", "test induced error")
	return s
}

func makeFalseStatusPingSource() *v1beta2.PingSource {
	return rtv1beta2.NewPingSource(pingSourceName, testNS, rtv1beta2.WithPingSourceSinkNotFound)
}

func makeUnknownStatusCronJobSource() *v1beta2.PingSource {
	cjs := rtv1beta2.NewPingSource(pingSourceName, testNS)
	cjs.Status.InitializeConditions()
	return cjs
}

func makeGenerationNotEqualPingSource() *v1beta2.PingSource {
	c := makeFalseStatusPingSource()
	c.Generation = currentGeneration
	c.Status.ObservedGeneration = outdatedGeneration
	return c
}

func makeReadyPingSource() *v1beta2.PingSource {
	u, _ := apis.ParseURL(sinkURI)
	return rtv1beta2.NewPingSource(pingSourceName, testNS,
		rtv1beta2.WithPingSourceSpec(v1beta2.PingSourceSpec{
			Schedule:    testSchedule,
			ContentType: testContentType,
			Data:        testData,
			SourceSpec: duckv1.SourceSpec{
				Sink: brokerDestv1,
			},
		}),
		rtv1beta2.WithInitPingSourceConditions,
		rtv1beta2.WithPingSourceDeployed,
		rtv1beta2.WithPingSourceCloudEventAttributes,
		rtv1beta2.WithPingSourceSink(u),
	)
}
func makeSubscriberKubernetesServiceAsUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      subscriberName,
			},
		},
	}
}

// FilterLabels generates the labels present on all resources representing the filter of the given
// Broker.
func FilterLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/brokerRole": "filter",
	}
}

func IngressLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/brokerRole": "ingress",
	}
}

// Create Ready Broker with proper annotations.
func ReadyBroker() *eventingv1.Broker {
	return NewBroker(brokerName, testNS,
		WithBrokerClass(eventing.MTChannelBrokerClassValue),
		WithBrokerConfig(config()),
		WithInitBrokerConditions,
		WithBrokerReady,
		WithChannelAddressAnnotation(triggerChannelURL),
		WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
		WithChannelKindAnnotation(triggerChannelKind),
		WithChannelNameAnnotation(triggerChannelName))
}

func makeDLSServiceAsUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      dlsName,
			},
		},
	}
}

func makeTriggerOIDCServiceAccount() *corev1.ServiceAccount {
	return auth.GetOIDCServiceAccountForResource(v1.SchemeGroupVersion.WithKind("Trigger"), metav1.ObjectMeta{
		Name:      triggerName,
		Namespace: testNS,
		UID:       triggerUID,
	})
}

func makeTriggerOIDCServiceAccountWithoutOwnerRef() *corev1.ServiceAccount {
	sa := auth.GetOIDCServiceAccountForResource(v1.SchemeGroupVersion.WithKind("Trigger"), metav1.ObjectMeta{
		Name:      triggerName,
		Namespace: testNS,
		UID:       triggerUID,
	})
	sa.OwnerReferences = nil

	return sa
}
