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

package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/kref"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/feature"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/auth"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/channel/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/inmemorychannel/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/subscription"
	"knative.dev/eventing/pkg/duck"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
)

const (
	subscriberName         = "subscriber"
	replyName              = "reply"
	channelName            = "origin"
	serviceName            = "service"
	dlcName                = "dlc"
	dlc2Name               = "dlc2"
	dlsName                = "dls"
	tlsSubscriberName      = "tls-subscriber"
	audienceSubscriberName = "audience-subscriber"

	subscriptionUID        = subscriptionName + "-abc-123"
	subscriptionName       = "testsubscription"
	testNS                 = "testnamespace"
	subscriptionGeneration = 1

	finalizerName = "subscriptions.messaging.knative.dev"
)

// subscriptions have: channel -> SUB -> subscriber -viaSub-> reply

var (
	channelDNS = duckv1.Addressable{
		URL: apis.HTTP("channel.mynamespace.svc." + network.GetClusterDomainName()),
	}

	subscriber = duckv1.Addressable{
		URL: apis.HTTP("subscriber.mynamespace.svc." + network.GetClusterDomainName()),
	}
	subscriberURI = subscriber.URL

	tlsSubscriberDNS     = "tls-subscriber.mynamespace.svc." + network.GetClusterDomainName()
	tlsSubscriberURI     = apis.HTTPS(tlsSubscriberDNS)
	tlsSubscriberCACerts = `-----BEGIN CERTIFICATE-----
MIIDLTCCAhWgAwIBAgIJAOjtl0zhGBvpMA0GCSqGSIb3DQEBCwUAMC0xEzARBgNV
BAoMCmlvLnN0cmltemkxFjAUBgNVBAMMDWNsdXN0ZXItY2EgdjAwHhcNMjAxMjIw
MDgzNzU4WhcNMjExMjIwMDgzNzU4WjAtMRMwEQYDVQQKDAppby5zdHJpbXppMRYw
FAYDVQQDDA1jbHVzdGVyLWNhIHYwMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEAy7rIo+UwJh5dL6PhUfDe9wRuLgOf1ZeZmabd++eLc2kWL1r6TO8X034n
CerkREfjF+MjDoK30z9xvEURThoSi20a4i/Cb39on9T0AgOr5qCSrqlN9n4KtRey
ZLnKKA5QyLAM6kzyyvIg4PVwWCWFTQSicDPzqd2OmH6jtogD50FkbaP7LcyrKnWf
64gcR9CCEAcrO8tJdhcZP2Slxg+RvupVjXK1rdZcI6/liZ3Jp4hzApSRN30x/8wU
5eJYAtzaeWUvJ0Yq/7BH7uY8J+2Hwh+shhi5K98HBAKeISwuIJEQrWmmUer8WGp1
IcBZqXbkd4dBXuFa0chO0gSKvzjKpQIDAQABo1AwTjAdBgNVHQ4EFgQUeascji1L
C2voPwDAlPL6iz8TzncwHwYDVR0jBBgwFoAUeascji1LC2voPwDAlPL6iz8Tzncw
DAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAIEL2uustCrpPas06LyoR
VR6QFHQJDcMgdL0CZFE46uLSgGupXO0ybmPP2ymBJ1zDNxx1qskNTwBsfBJLBAj6
8LfJmhfw98QK8YQDJ/Xhx3fcVxjn6NjJ3RYOyb5bqSIGGCQZRmbMjerf71KMhP3X
rdYg2hVoCvfRcfP2G0jbWtMRK4+MlB3oEvhIvQQW1dw4sohw32HaNJnzb7dErEDB
Ha2zVM47CcNezdWYUD5NQzFqCRypgrIONafQI2S+Ck7aKOiqF03QSug4wizRbKhT
uYpQg59dUIOBebg0roRF326H2x6kFGn5L2o+TROrZeeXT8vyIl2R33o3E+ULpuw+
Vw==
-----END CERTIFICATE-----
`
	tlsSubscriber = duckv1.Addressable{
		URL:     tlsSubscriberURI,
		CACerts: &tlsSubscriberCACerts,
	}

	audienceSubscriberDNS      = "subscriber-with-audience.mynamespace.svc." + network.GetClusterDomainName()
	audienceSubscriberURI      = apis.HTTPS(audienceSubscriberDNS)
	audienceSubscriberAudience = "my-audience"
	audienceSubscriber         = duckv1.Addressable{
		URL:      audienceSubscriberURI,
		Audience: &audienceSubscriberAudience,
	}

	reply = duckv1.Addressable{
		URL: apis.HTTP("reply.mynamespace.svc." + network.GetClusterDomainName()),
	}
	replyURI = reply.URL

	serviceDNS = serviceName + "." + testNS + ".svc." + network.GetClusterDomainName()
	serviceURI = apis.HTTP(serviceDNS)
	service    = duckv1.Addressable{
		URL: serviceURI,
	}

	dlc = duckv1.Addressable{
		URL: apis.HTTP("dlc.mynamespace.svc." + network.GetClusterDomainName()),
	}
	dlcStatus = &dlc

	dlc2 = duckv1.Addressable{
		URL: apis.HTTP("dlc2.mynamespace.svc." + network.GetClusterDomainName()),
	}

	dls = duckv1.Addressable{
		URL: apis.HTTP("dls.mynamespace.svc." + network.GetClusterDomainName()),
	}
	dlsURI = &dls

	subscriberGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "Subscriber",
	}

	nonAddressableGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1",
		Kind:    "Trigger",
	}

	serviceGVK = metav1.GroupVersionKind{
		Version: "v1",
		Kind:    "Service",
	}

	imcV1GVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "InMemoryChannel",
	}

	channelV1GVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1",
		Kind:    "Channel",
	}

	imcV1KRef = duckv1.KReference{
		APIVersion: "messaging.knative.dev/v1",
		Kind:       "InMemoryChannel",
		Namespace:  testNS,
		Name:       channelName,
	}
	sinkURL = apis.HTTP("example.com")
	sink    = &duckv1.Addressable{
		Name: &sinkURL.Scheme,
		URL:  sinkURL,
	}
)

func TestAllCases(t *testing.T) {
	linear := eventingduck.BackoffPolicyLinear

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
			Name: "subscription goes ready",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Reply
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(reply),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						UID:           subscriptionUID,
						Generation:    0,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}, {
						UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						Generation:    1,
						SubscriberURI: apis.HTTP("call2"),
						ReplyURI:      apis.HTTP("sink2"),
					}}),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID:                subscriptionUID,
						ObservedGeneration: 0,
						Ready:              "True",
					}, {
						UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 1,
						Ready:              "True",
					}}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					// - Status Update -
					MarkSubscriptionReady,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "subscription goes ready with subscriber in different namespace",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS+"-2"),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS+"-2",
					WithUnstructuredAddressable(subscriber),
				),
				// Reply
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(reply),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						UID:           subscriptionUID,
						Generation:    0,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}, {
						UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						Generation:    1,
						SubscriberURI: apis.HTTP("call2"),
						ReplyURI:      apis.HTTP("sink2"),
					}}),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID:                subscriptionUID,
						ObservedGeneration: 0,
						Ready:              "True",
					}, {
						UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 1,
						Ready:              "True",
					}}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS+"-2"),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					// - Status Update -
					MarkSubscriptionReady,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "subscription goes ready with reply in different namespace",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS+"-2"),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Reply
				NewInMemoryChannel(replyName, testNS+"-2",
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(reply),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						UID:           subscriptionUID,
						Generation:    0,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}, {
						UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						Generation:    1,
						SubscriberURI: apis.HTTP("call2"),
						ReplyURI:      apis.HTTP("sink2"),
					}}),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID:                subscriptionUID,
						ObservedGeneration: 0,
						Ready:              "True",
					}, {
						UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 1,
						Ready:              "True",
					}}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS+"-2"),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					// - Status Update -
					MarkSubscriptionReady,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "subscription goes ready without api version",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.KReferenceGroup: feature.Enabled,
			}),
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRefUsingGroup(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Reply
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(reply),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						UID:           subscriptionUID,
						Generation:    0,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}, {
						UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						Generation:    1,
						SubscriberURI: apis.HTTP("call2"),
						ReplyURI:      apis.HTTP("sink2"),
					}}),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID:                subscriptionUID,
						ObservedGeneration: 0,
						Ready:              "True",
					}, {
						UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 1,
						Ready:              "True",
					}}),
				),
				// Subscriber CRD
				eventingtesting.NewCustomResourceDefinition("subscribers.messaging.knative.dev",
					eventingtesting.WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:    "v1beta1",
						Storage: false,
					}, {
						Name:    "v1",
						Storage: true,
					}}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRefUsingGroup(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					// - Status Update -
					MarkSubscriptionReady,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "subscription goes ready with both api version and group",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.KReferenceGroup: feature.Enabled,
			}),
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRefUsingApiVersionAndGroup(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Reply
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(reply),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						UID:           subscriptionUID,
						Generation:    0,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}, {
						UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						Generation:    1,
						SubscriberURI: apis.HTTP("call2"),
						ReplyURI:      apis.HTTP("sink2"),
					}}),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID:                subscriptionUID,
						ObservedGeneration: 0,
						Ready:              "True",
					}, {
						UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 1,
						Ready:              "True",
					}}),
				),
				// IMC CRD
				eventingtesting.NewCustomResourceDefinition("subscribers.messaging.knative.dev",
					eventingtesting.WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:    "v1beta1",
						Storage: false,
					}, {
						Name:    "v1",
						Storage: true,
					}}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRefUsingApiVersionAndGroup(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					// - Status Update -
					MarkSubscriptionReady,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "subscription goes ready with Channel.Group and Subscriber.Ref.Group",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.KReferenceGroup: feature.Enabled,
			}),
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannelUsingGroup(imcV1GVK, channelName),
					WithSubscriptionSubscriberRefUsingGroup(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Reply
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(reply),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						UID:           subscriptionUID,
						Generation:    0,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}, {
						UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						Generation:    1,
						SubscriberURI: apis.HTTP("call2"),
						ReplyURI:      apis.HTTP("sink2"),
					}}),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID:                subscriptionUID,
						ObservedGeneration: 0,
						Ready:              "True",
					}, {
						UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 1,
						Ready:              "True",
					}}),
				),
				eventingtesting.NewCustomResourceDefinition("subscribers.messaging.knative.dev",
					eventingtesting.WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:    "v1beta1",
						Storage: false,
					}, {
						Name:    "v1",
						Storage: true,
					}}),
				),
				eventingtesting.NewCustomResourceDefinition("inmemorychannels.messaging.knative.dev",
					eventingtesting.WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:    "v1beta1",
						Storage: false,
					}, {
						Name:    "v1",
						Storage: true,
					}}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannelUsingGroup(imcV1GVK, channelName),
					WithSubscriptionSubscriberRefUsingGroup(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					// - Status Update -
					MarkSubscriptionReady,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "subscription goes ready with Channel.Group and Channel.APIVersion",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.KReferenceGroup: feature.Enabled,
			}),
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannelUsingApiVersionAndGroup(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Reply
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(reply),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						UID:           subscriptionUID,
						Generation:    0,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}, {
						UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						Generation:    1,
						SubscriberURI: apis.HTTP("call2"),
						ReplyURI:      apis.HTTP("sink2"),
					}}),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID:                subscriptionUID,
						ObservedGeneration: 0,
						Ready:              "True",
					}, {
						UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 1,
						Ready:              "True",
					}}),
				),
				eventingtesting.NewCustomResourceDefinition("subscribers.messaging.knative.dev",
					eventingtesting.WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:    "v1beta1",
						Storage: false,
					}, {
						Name:    "v1",
						Storage: true,
					}}),
				),
				eventingtesting.NewCustomResourceDefinition("inmemorychannels.messaging.knative.dev",
					eventingtesting.WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:    "v1beta1",
						Storage: false,
					}, {
						Name:    "v1",
						Storage: true,
					}}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannelUsingApiVersionAndGroup(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					// - Status Update -
					MarkSubscriptionReady,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		}, {
			Name: "subscriber with CA certs",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, tlsSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
				),
				// Subscriber
				NewUnstructured(subscriberGVK, tlsSubscriberName, testNS,
					WithUnstructuredAddressableTLS(tlsSubscriberDNS, &tlsSubscriberCACerts),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						SubscriberURI: tlsSubscriberURI,
					}}),
					WithInMemoryChannelAddress(channelDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, tlsSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&tlsSubscriber),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						SubscriberURI:     tlsSubscriberURI,
						SubscriberCACerts: &tlsSubscriberCACerts,
					},
				}),
			},
		}, {
			Name: "reply with CA certs",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, tlsSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Reply
				NewInMemoryChannel(tlsSubscriberName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddressHTTPS(duckv1.Addressable{
						URL:     tlsSubscriberURI,
						CACerts: &tlsSubscriberCACerts,
					}),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						SubscriberURI: subscriberURI,
						ReplyURI:      tlsSubscriberURI,
					}}),
					WithInMemoryChannelAddress(subscriber),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, tlsSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&tlsSubscriber),
					// - Status Update -
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						SubscriberURI: subscriberURI,
						ReplyURI:      tlsSubscriberURI,
						ReplyCACerts:  &tlsSubscriberCACerts,
					},
				}),
			},
		}, {
			Name: "subscriber with Audience",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, audienceSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
				),
				// Subscriber
				NewUnstructured(subscriberGVK, audienceSubscriberName, testNS,
					WithUnstructuredAddressable(audienceSubscriber),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						SubscriberURI: audienceSubscriberURI,
					}}),
					WithInMemoryChannelAddress(channelDNS),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, audienceSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&audienceSubscriber),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						SubscriberURI:      audienceSubscriberURI,
						SubscriberAudience: &audienceSubscriberAudience,
					},
				}),
			},
		}, {
			Name: "reply with Audience",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, audienceSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Reply
				NewInMemoryChannel(audienceSubscriberName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddressHTTPS(duckv1.Addressable{
						URL:      audienceSubscriberURI,
						Audience: &audienceSubscriberAudience,
					}),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						SubscriberURI: subscriberURI,
						ReplyURI:      audienceSubscriberURI,
					}}),
					WithInMemoryChannelAddress(subscriber),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, audienceSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&audienceSubscriber),
					// - Status Update -
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						SubscriberURI: subscriberURI,
						ReplyURI:      audienceSubscriberURI,
						ReplyAudience: &audienceSubscriberAudience,
					},
				}),
			},
		}, {
			Name: "no patch on subscriber without CA certs",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, tlsSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					MarkSubscriptionReady,
				),
				// Subscriber
				NewUnstructured(subscriberGVK, tlsSubscriberName, testNS,
					WithUnstructuredAddressableTLS(tlsSubscriberDNS, nil),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						UID:           subscriptionUID,
						SubscriberURI: tlsSubscriberURI,
					}}),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID:                subscriptionUID,
						ObservedGeneration: 0,
						Ready:              "True",
					}}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, tlsSubscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					MarkSubscriptionReady,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionPhysicalSubscriptionSubscriber(&duckv1.Addressable{
						URL: tlsSubscriberURI,
					}),
				),
			}},
			WantPatches: nil,
		}, {
			Name: "channel does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, channelReferenceFailed, "Failed to get Spec.Channel or backing channel: inmemorychannels.messaging.knative.dev %q not found", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionReferencesResolvedUnknown(channelReferenceFailed, fmt.Sprintf("Failed to get Spec.Channel or backing channel: inmemorychannels.messaging.knative.dev %q not found", channelName)),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "channel does not exist - fail status update",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "subscriptions"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, subscriptionUpdateStatusFailed, "Failed to update status for %q: inducing failure for update subscriptions", subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionReferencesResolvedUnknown(channelReferenceFailed, fmt.Sprintf("Failed to get Spec.Channel or backing channel: inmemorychannels.messaging.knative.dev %q not found", channelName)),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscriber is not addressable",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: address not set for Kind = Subscriber, Namespace = testnamespace, Name = subscriber, APIVersion = messaging.knative.dev/v1, Group = , Address = "),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionReferencesNotResolved(subscriberResolveFailed, "Failed to resolve spec.subscriber: address not set for Kind = Subscriber, Namespace = testnamespace, Name = subscriber, APIVersion = messaging.knative.dev/v1, Group = , Address = "),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscriber does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "SubscriberResolveFailed", "Failed to resolve spec.subscriber: failed to get object testnamespace/subscriber: subscribers.messaging.knative.dev %q not found", subscriberName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionReferencesNotResolved(subscriberResolveFailed, `Failed to resolve spec.subscriber: failed to get object testnamespace/subscriber: subscribers.messaging.knative.dev "subscriber" not found`),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "reply does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber)),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "ReplyResolveFailed", `Failed to resolve spec.reply: failed to get object testnamespace/reply: inmemorychannels.messaging.knative.dev %q not found`, replyName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionReferencesNotResolved(replyResolveFailed, fmt.Sprintf(`Failed to resolve spec.reply: failed to get object testnamespace/reply: inmemorychannels.messaging.knative.dev %q not found`, replyName)),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "reply is not addressable",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(nonAddressableGVK, replyName, testNS), // reply will be a nonAddressableGVK for this test
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
				),
				NewUnstructured(nonAddressableGVK, replyName, testNS),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, replyResolveFailed, "Failed to resolve spec.reply: address not set for Kind = Trigger, Namespace = testnamespace, Name = reply, APIVersion = eventing.knative.dev/v1, Group = , Address = "),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionReply(nonAddressableGVK, replyName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionReferencesNotResolved(replyResolveFailed, "Failed to resolve spec.reply: address not set for Kind = Trigger, Namespace = testnamespace, Name = reply, APIVersion = eventing.knative.dev/v1, Group = , Address = "),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc, valid channel+subscriber",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc, valid channel+subscriber+missing delivery",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlsName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "DeadLetterSinkResolveFailed", `Failed to resolve spec.delivery.deadLetterSink: failed to get object testnamespace/dls: subscribers.messaging.knative.dev "dls" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlsName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionReferencesNotResolved("DeadLetterSinkResolveFailed", `Failed to resolve spec.delivery.deadLetterSink: failed to get object testnamespace/dls: subscribers.messaging.knative.dev "dls" not found`),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1beta imc, valid channel+subscriber+delivery",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlsName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewUnstructured(subscriberGVK, dlsName, testNS,
					WithUnstructuredAddressable(dls),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlsName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionDeadLetterSink(dlsURI),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI, Delivery: &eventingduck.DeliverySpec{DeadLetterSink: &duckv1.Destination{URI: apis.HTTP("dls.mynamespace.svc.cluster.local")}}},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 channel+v1 imc backing channel+subscriber",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithBackingChannelObjRef(&imcV1KRef),
					WithBackingChannelReady,
					WithChannelAddress(sink),
					WithChannelDLSUnknown(),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
					WithInMemoryChannelReady(duckv1.Addressable{URL: apis.HTTP("example.com")}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 channel+backing v1 imc channel not ready+subscriber",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewChannel(channelName, testNS,
					WithInitChannelConditions,
					WithBackingChannelObjRef(&imcV1KRef),
					WithBackingChannelReady,
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReadySubscriber(subscriptionUID),
					WithInMemoryChannelReady(duckv1.Addressable{URL: apis.HTTP("example.com")}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeWarning, "ChannelReferenceFailed", "Failed to get Spec.Channel or backing channel: channel is not ready."),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(channelV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionReferencesResolvedUnknown("ChannelReferenceFailed", "Failed to get Spec.Channel or backing channel: channel is not ready."),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc channel+reply",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(reply),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, ReplyURI: replyURI, SubscriberURI: subscriberURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc+subscriber+reply",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
				NewInMemoryChannel(replyName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(reply),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc+valid remove reply",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					WithInMemoryChannelReadySubscriberAndGeneration(subscriptionUID, subscriptionGeneration),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionGeneration(subscriptionGeneration),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionStatusObservedGeneration(subscriptionGeneration),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, Generation: subscriptionGeneration, SubscriberURI: subscriberURI},
				}),
			},
		}, {
			Name: "v1 imc+subscriber as service",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: subscriptionUID, SubscriberURI: serviceURI},
				}),
				patchFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "v1 imc+two subscribers for a channel",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
				),
				// an already rec'ed subscription
				NewSubscription("b-"+subscriptionName, testNS,
					WithSubscriptionUID("b-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelReadySubscriber("b-"+subscriptionUID),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: "a-" + subscriptionUID, SubscriberURI: serviceURI},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		}, {
			Name: "v1 imc+two subscribers for a channel - update delivery",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlsName, testNS),
					WithSubscriptionDeadLetterSink(dlsURI),
				),
				NewUnstructured(subscriberGVK, dlsName, testNS,
					WithUnstructuredAddressable(dls),
				),
				// an already rec'ed subscription
				NewSubscription("b-"+subscriptionName, testNS,
					WithSubscriptionUID("b-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: "a-" + subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelReadySubscriber("b-"+subscriptionUID),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithSubscriptionDeliveryRef(subscriberGVK, dlsName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionDeadLetterSink(dlsURI),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{UID: "a-" + subscriptionUID, SubscriberURI: serviceURI, Delivery: &eventingduck.DeliverySpec{DeadLetterSink: &duckv1.Destination{URI: apis.HTTP("dls.mynamespace.svc.cluster.local")}}},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc+two subscribers for a channel - update delivery - full delivery spec",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlsName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.String("PT1S"),
					}),
				),
				NewUnstructured(subscriberGVK, dlsName, testNS,
					WithUnstructuredAddressable(dls),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelReadySubscriber("b-"+subscriptionUID),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlsName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.String("PT1S"),
					}),
					WithSubscriptionDeadLetterSink(dlsURI),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						UID:           "a-" + subscriptionUID,
						SubscriberURI: serviceURI,
						Delivery: &eventingduck.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								URI: apis.HTTP("dls.mynamespace.svc.cluster.local"),
							},
							Retry:         pointer.Int32(10),
							BackoffPolicy: &linear,
							BackoffDelay:  pointer.String("PT1S"),
						},
					},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc - delivery defaulting - full delivery spec",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
				),
				NewUnstructured(subscriberGVK, dlcName, testNS,
					WithUnstructuredAddressable(dlc),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelDelivery(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlcName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.String("PT1S"),
					}),
					WithInMemoryChannelStatusDLS(dlcStatus),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionDeadLetterSink(dlcStatus),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						UID:           "a-" + subscriptionUID,
						SubscriberURI: serviceURI,
						Delivery: &eventingduck.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								URI: apis.HTTP("dlc.mynamespace.svc.cluster.local"),
							},
							Retry:         pointer.Int32(10),
							BackoffPolicy: &linear,
							BackoffDelay:  pointer.String("PT1S"),
						},
					},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc - delivery defaulting - optional features",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.DeliveryTimeout:    feature.Enabled,
				feature.DeliveryRetryAfter: feature.Enabled,
			}),
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
				),
				NewUnstructured(subscriberGVK, dlcName, testNS,
					WithUnstructuredAddressable(dlc),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelDelivery(&eventingduck.DeliverySpec{
						Timeout:       pointer.String("PT1S"),
						RetryAfterMax: pointer.String("PT2S"),
					}),
					WithInMemoryChannelStatusDLS(dlcStatus),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						UID:           "a-" + subscriptionUID,
						SubscriberURI: serviceURI,
						Delivery: &eventingduck.DeliverySpec{
							Timeout:       pointer.String("PT1S"),
							RetryAfterMax: pointer.String("PT2S"),
						},
					},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc - no dls on imc nor subscription",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
				),
				NewUnstructured(subscriberGVK, dlcName, testNS,
					WithUnstructuredAddressable(dlc),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						UID:           "a-" + subscriptionUID,
						SubscriberURI: serviceURI,
					},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc - error on channel status dls uri, subscription not ready",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
				),
				NewUnstructured(subscriberGVK, dlcName, testNS,
					WithUnstructuredAddressable(dlc),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelDelivery(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlcName,
								Namespace:  testNS,
							},
						},
					}),
					WithInMemoryChannelStatusDLS(nil),
					WithInMemoryChannelReady(channelDNS),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeWarning, "DeadLetterSinkResolveFailed", "channel %s didn't set status.deadLetterSinkURI", channelName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					WithSubscriptionReferencesNotResolved("DeadLetterSinkResolveFailed", fmt.Sprintf("channel %s didn't set status.deadLetterSinkURI", channelName)),
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
		},
		{
			Name: "v1 imc - don't default delivery - full delivery spec",
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlsName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.String("PT1S"),
					}),
				),
				NewUnstructured(subscriberGVK, dlsName, testNS,
					WithUnstructuredAddressable(dls),
				),
				NewUnstructured(subscriberGVK, dlc2Name, testNS,
					WithUnstructuredAddressable(dlc2),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelDelivery(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlc2Name,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32(20),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.String("PT10S"),
					}),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: subscriberGVK.Group + "/" + subscriberGVK.Version,
								Kind:       subscriberGVK.Kind,
								Name:       dlsName,
								Namespace:  testNS,
							},
						},
						Retry:         pointer.Int32(10),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.String("PT1S"),
					}),
					WithSubscriptionDeadLetterSink(dlsURI),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						UID:           "a-" + subscriptionUID,
						SubscriberURI: serviceURI,
						Delivery: &eventingduck.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								URI: apis.HTTP("dls.mynamespace.svc.cluster.local"),
							},
							Retry:         pointer.Int32(10),
							BackoffPolicy: &linear,
							BackoffDelay:  pointer.String("PT1S"),
						},
					},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc - don't default delivery - optional features",
			Ctx: feature.ToContext(context.TODO(), feature.Flags{
				feature.DeliveryTimeout:    feature.Enabled,
				feature.DeliveryRetryAfter: feature.Enabled,
			}),
			Objects: []runtime.Object{
				NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						Timeout:       pointer.String("PT1S"),
						RetryAfterMax: pointer.String("PT2S"),
					}),
				),
				NewUnstructured(subscriberGVK, dlsName, testNS,
					WithUnstructuredAddressable(dls),
				),
				NewUnstructured(subscriberGVK, dlc2Name, testNS,
					WithUnstructuredAddressable(dlc2),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelSubscribers(nil),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelReadySubscriber("a-"+subscriptionUID),
					WithInMemoryChannelDelivery(&eventingduck.DeliverySpec{
						Timeout:       pointer.String("PT10S"),
						RetryAfterMax: pointer.String("PT20S"),
					}),
				),
				NewService(serviceName, testNS),
			},
			Key:     testNS + "/" + "a-" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", "a-"+subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription("a-"+subscriptionName, testNS,
					WithSubscriptionUID("a-"+subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(serviceGVK, serviceName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitSubscriptionConditions,
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionOIDCIdentityCreatedSucceededBecauseOIDCFeatureDisabled(),
					WithSubscriptionDeliverySpec(&eventingduck.DeliverySpec{
						Timeout:       pointer.String("PT1S"),
						RetryAfterMax: pointer.String("PT2S"),
					}),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, []eventingduck.SubscriberSpec{
					{
						UID:           "a-" + subscriptionUID,
						SubscriberURI: serviceURI,
						Delivery: &eventingduck.DeliverySpec{
							Timeout:       pointer.String("PT1S"),
							RetryAfterMax: pointer.String("PT2S"),
						},
					},
				}),
				patchFinalizers(testNS, "a-"+subscriptionName),
			},
		},
		{
			Name: "v1 imc+deleted - channel patch succeeded",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionDeleted,
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI},
					}),
				),
			},
			Key:     testNS + "/" + subscriptionName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
				Eventf(corev1.EventTypeNormal, "SubscriberRemoved", "Subscription was removed from channel \"origin\""),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, nil),
				patchRemoveFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "subscription not deleted - channel patch fails",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionDeleted,
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI},
					}),
					WithInMemoryChannelReadySubscriber(subscriptionUID),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("patch", "inmemorychannels"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "PhysicalChannelSyncFailed", fmt.Sprintf("Failed to synchronize to channel %q: %s", channelName, "inducing failure for patch inmemorychannels")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					MarkNotAddedToChannel("PhysicalChannelSyncFailed", "Failed to sync physical Channel: inducing failure for patch inmemorychannels"),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionDeleted,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchSubscribers(testNS, channelName, nil),
			},
		}, {
			Name: "subscription deleted - channel does not exist",
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithSubscriptionReply(imcV1GVK, replyName, testNS),
					WithInitSubscriptionConditions,
					MarkSubscriptionReady,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&service),
					WithSubscriptionDeleted,
				),
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
			},
			Key: testNS + "/" + subscriptionName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", subscriptionName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchRemoveFinalizers(testNS, subscriptionName),
			},
		}, {
			Name: "OIDC: creates OIDC service account",
			Key:  testNS + "/" + subscriptionName,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Objects: []runtime.Object{
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkSubscriptionReady,
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						SubscriberURI: subscriberURI,
						UID:           subscriptionUID,
					}}),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID: subscriptionUID,
					}}),
				),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					// - Status Update -
					MarkSubscriptionReady,
					WithSubscriptionOIDCIdentityCreatedSucceeded(),
					WithSubscriptionOIDCServiceAccountName(makeSubscriptionOIDCServiceAccount().Name),
				),
			}},
			WantCreates: []runtime.Object{
				makeSubscriptionOIDCServiceAccount(),
			},
		}, {
			Name: "OIDC: Subscription not ready on invalid OIDC service account",
			Key:  testNS + "/" + subscriptionName,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			Objects: []runtime.Object{
				makeSubscriptionOIDCServiceAccountWithoutOwnerRef(),
				NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					MarkReferencesResolved,
					MarkAddedToChannel,
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
				),
				// Subscriber
				NewUnstructured(subscriberGVK, subscriberName, testNS,
					WithUnstructuredAddressable(subscriber),
				),
				// Channel
				NewInMemoryChannel(channelName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelReady(channelDNS),
					WithInMemoryChannelSubscribers([]eventingduck.SubscriberSpec{{
						SubscriberURI: subscriberURI,
						UID:           subscriptionUID,
					}}),
					WithInMemoryChannelAddress(channelDNS),
					WithInMemoryChannelStatusSubscribers([]eventingduck.SubscriberStatus{{
						UID: subscriptionUID,
					}}),
				),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewSubscription(subscriptionName, testNS,
					WithSubscriptionUID(subscriptionUID),
					WithSubscriptionChannel(imcV1GVK, channelName),
					WithSubscriptionSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitSubscriptionConditions,
					WithSubscriptionFinalizers(finalizerName),
					WithSubscriptionPhysicalSubscriptionSubscriber(&subscriber),
					WithSubscriptionPhysicalSubscriptionReply(&reply),
					// - Status Update -
					MarkAddedToChannel,
					MarkReferencesResolved,
					WithSubscriptionOIDCIdentityCreatedFailed("Unable to resolve service account for OIDC authentication", fmt.Sprintf("service account %s not owned by Subscription %s", makeSubscriptionOIDCServiceAccountWithoutOwnerRef().Name, subscriptionName)),
					WithSubscriptionOIDCServiceAccountName(makeSubscriptionOIDCServiceAccount().Name),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", fmt.Sprintf("service account %s not owned by Subscription %s", makeSubscriptionOIDCServiceAccountWithoutOwnerRef().Name, subscriptionName)),
			},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			dynamicClientSet:     dynamicclient.Get(ctx),
			kubeclient:           fakekubeclient.Get(ctx),
			subscriptionLister:   listers.GetSubscriptionLister(),
			channelLister:        listers.GetMessagingChannelLister(),
			channelableTracker:   duck.NewListableTrackerFromTracker(ctx, channelable.Get, tracker.New(func(types.NamespacedName) {}, 0)),
			destinationResolver:  resolver.NewURIResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0)),
			kreferenceResolver:   kref.NewKReferenceResolver(listers.GetCustomResourceDefinitionLister()),
			tracker:              &FakeTracker{},
			serviceAccountLister: listers.GetServiceAccountLister(),
		}
		return subscription.NewReconciler(ctx, logger,
			eventingclient.Get(ctx), listers.GetSubscriptionLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))
}

func WithSubscriptionDeliverySpec(d *eventingduck.DeliverySpec) SubscriptionOption {
	return func(v *messagingv1.Subscription) {
		v.Spec.Delivery = d
	}
}

func patchSubscribers(namespace, name string, subscribers []eventingduck.SubscriberSpec) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace

	var spec string
	if subscribers != nil {
		b, err := json.Marshal(subscribers)
		if err != nil {
			return action
		}
		ss := make([]map[string]interface{}, 0)
		err = json.Unmarshal(b, &ss)
		if err != nil {
			return action
		}
		subs, err := json.Marshal(ss)
		if err != nil {
			return action
		}
		spec = fmt.Sprintf(`{"subscribers":%s}`, subs)
	} else {
		spec = `{"subscribers":null}`
	}

	patch := `{"spec":` + spec + `}`
	action.Patch = []byte(patch)
	return action
}

func patchFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizerName + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func patchRemoveFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":[],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func makeSubscriptionOIDCServiceAccount() *corev1.ServiceAccount {
	return auth.GetOIDCServiceAccountForResource(messagingv1.SchemeGroupVersion.WithKind("Subscription"), metav1.ObjectMeta{
		Name:      subscriptionName,
		Namespace: testNS,
		UID:       subscriptionUID,
	})
}

func makeSubscriptionOIDCServiceAccountWithoutOwnerRef() *corev1.ServiceAccount {
	sa := auth.GetOIDCServiceAccountForResource(messagingv1.SchemeGroupVersion.WithKind("Subscription"), metav1.ObjectMeta{
		Name:      subscriptionName,
		Namespace: testNS,
		UID:       subscriptionUID,
	})
	sa.OwnerReferences = nil

	return sa
}
