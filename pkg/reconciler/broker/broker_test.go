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
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/feature"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1a1addr "knative.dev/pkg/client/injection/ducks/duck/v1alpha1/addressable"
	v1b1addr "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	"knative.dev/pkg/tracker"

	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	systemNS        = "knative-testing"
	testNS          = "test-namespace"
	brokerName      = "test-broker"
	sinkName        = "test-sink"
	dlsName         = "test-dls"
	alternateDLS    = "test-dls-alternate"
	deliveryRetries = 3

	configMapName = "test-configmap"

	triggerChannelAPIVersion = "messaging.knative.dev/v1"
	triggerChannelKind       = "InMemoryChannel"
	triggerChannelName       = "test-broker-kne-trigger"

	imcSpec = `
apiVersion: "messaging.knative.dev/v1"
kind: "InMemoryChannel"
`
)

var (
	testKey = fmt.Sprintf("%s/%s", testNS, brokerName)

	triggerChannelHostname = network.GetServiceHostname("foo", "bar")
	triggerChannelURL      = fmt.Sprintf("http://%s", triggerChannelHostname)

	filterServiceName  = "broker-filter"
	ingressServiceName = "broker-ingress"

	brokerAddress = &apis.URL{
		Scheme: "http",
		Host:   network.GetServiceHostname(ingressServiceName, systemNS),
		Path:   fmt.Sprintf("/%s/%s", testNS, brokerName),
	}

	brokerDestv1 = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1",
		},
	}

	DLSAddress = &apis.URL{
		Scheme: "http",
		Host:   network.GetServiceHostname(ingressServiceName, systemNS),
		Path:   fmt.Sprintf("/%s/%s", testNS, dlsName),
	}

	sinkSVCDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       dlsName,
			Kind:       "Service",
			APIVersion: "v1",
			Namespace:  testNS,
		},
	}

	alternateDLSDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       alternateDLS,
			Kind:       "Service",
			APIVersion: "v1",
			Namespace:  testNS,
		},
	}

	dlsURI, _ = apis.ParseURL("http://test-dls.test-namespace.svc.cluster.local")
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
			Name: "Broker not found",
			Key:  testKey,
		}, {
			Name: "Broker is being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithBrokerDeletionTimestamp),
				imcConfigMap(),
			},
		}, {
			Name: "nil config",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "failed to find channelTemplate"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: failed to find channelTemplate")),
			}},
			// This returns an internal error, so it emits an Error
			WantErr: true,
		}, {
			Name: "nil config, missing name",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(&duckv1.KReference{Kind: "ConfigMap", APIVersion: "v1"}),
					WithInitBrokerConditions),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-broker": missing field(s): spec.config.name`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(&duckv1.KReference{Kind: "ConfigMap", APIVersion: "v1"}),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: Broker.Spec.Config name and namespace are required")),
			}},
			// This returns an internal error, so it emits an Error
			WantErr: true,
		}, {
			Name: "Config not found",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithTriggerChannelFailed("ChannelTemplateFailed", `Error on setting up the ChannelTemplate: configmap "test-configmap" not found`)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `configmap "test-configmap" not found`),
			},
			WantErr: true,
		}, {
			Name: "Trigger Channel.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				imcConfigMap(),
			},
			WantCreates: []runtime.Object{
				createChannel(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithTriggerChannelFailed("ChannelFailure",
						"failed to create channel "+
							testNS+"/"+triggerChannelName+
							": inducing failure for create inmemorychannels")),
			}},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "inmemorychannels"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "failed to reconcile trigger channel: "+
					"failed to create channel "+
					testNS+"/"+triggerChannelName+
					": inducing failure for create inmemorychannels"),
			},
			WantErr: true,
		}, {
			Name: "Trigger Channel.Create no address",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				imcConfigMap(),
			},
			WantCreates: []runtime.Object{
				createChannel(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithBrokerConfig(config()),
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
		}, {
			Name: "Trigger Channel.Create no host in the url",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createChannelNoHostInUrl(testNS),
				imcConfigMap(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
		}, {
			Name: "nil config, not a configmap",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(&duckv1.KReference{Kind: "Deployment", APIVersion: "v1", Name: "test"}),
					WithInitBrokerConditions),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `Broker.Spec.Config configuration not supported, only [kind: ConfigMap, apiVersion: v1]`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(&duckv1.KReference{Kind: "Deployment", APIVersion: "v1", Name: "test"}),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: Broker.Spec.Config configuration not supported, only [kind: ConfigMap, apiVersion: v1]")),
			}},
			// This returns an internal error, so it emits an Error
			WantErr: true,
		}, {
			Name: "Trigger Channel is not yet Addressable",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				imcConfigMap(),
				createChannel(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
		}, {
			Name: "Trigger Channel endpoints fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				imcConfigMap(),
				createChannel(withChannelReady),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithFilterFailed("ServiceFailure", `endpoints "broker-filter" not found`)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `endpoints "broker-filter" not found`),
			},
			WantErr: true,
		}, {
			Name: "Successful Reconciliation",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createChannel(withChannelReady),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReady,
					WithBrokerAddressURI(brokerAddress),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithDLSNotConfigured()),
			}},
		}, {
			Name: "Successful Reconciliation. Using legacy channel template config element.",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createChannel(withChannelReady),
				imcConfigMapLegacy(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReady,
					WithBrokerAddressURI(brokerAddress),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithDLSNotConfigured()),
			}},
		}, {
			Name: "Successful Reconciliation, status update fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithInitBrokerConditions),
				createChannel(withChannelReady),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "brokers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReady,
					WithBrokerAddressURI(brokerAddress),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithDLSNotConfigured()),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-broker": inducing failure for update brokers`),
			},
			WantErr: true,
		}, {
			Name: "Error broker, status with non existent DLS",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithDeadLeaderSink(brokerDestv1.Ref, ""),
					WithInitBrokerConditions),
				createChannel(withChannelDeadLetterSink(brokerDestv1)),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithInitBrokerConditions,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithDeadLeaderSink(brokerDestv1.Ref, ""),
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
		}, {
			Name: "valid Broker with DLS",
			Key:  testKey,
			Objects: []runtime.Object{
				makeDLSServiceAsUnstructured(),
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithDeadLeaderSink(sinkSVCDest.Ref, ""),
					WithInitBrokerConditions),
				createChannel(withChannelReady, withChannelDeadLetterSink(sinkSVCDest)),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReadyWithDLS,
					WithDeadLeaderSink(sinkSVCDest.Ref, ""),
					WithBrokerAddressURI(brokerAddress),
					WithBrokerStatusDLSURI(dlsURI),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName)),
			}},
			WantErr: false,
		}, {
			Name: "valid Broker with DLS is updated with new DLS, needs to propagate to channel",
			Key:  testKey,
			Objects: []runtime.Object{
				makeDLSServiceAsUnstructured(),
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithDeadLeaderSink(sinkSVCDest.Ref, ""),
					WithInitBrokerConditions),
				createChannel(withChannelReady, withChannelDeadLetterSink(alternateDLSDest)),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReadyWithDLS,
					WithDeadLeaderSink(sinkSVCDest.Ref, ""),
					WithBrokerAddressURI(brokerAddress),
					WithBrokerStatusDLSURI(dlsURI),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName)),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				makeChannelDLSRefNamePatch(sinkSVCDest.Ref.Name),
			},
		}, {
			Name: "valid Broker with no delivery is updated to use retries, needs to propagate to channel",
			Key:  testKey,
			Objects: []runtime.Object{
				makeDLSServiceAsUnstructured(),
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerDeliveryRetries(deliveryRetries),
					WithInitBrokerConditions),
				createChannel(withChannelReady),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReady,
					WithBrokerDeliveryRetries(deliveryRetries),
					WithBrokerAddressURI(brokerAddress),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName)),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				makeChannelDeliveryRetryPatch(deliveryRetries),
			},
		}, {
			Name: "TLS permissive",
			Key:  testKey,
			Objects: []runtime.Object{
				makeDLSServiceAsUnstructured(),
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithDeadLeaderSink(sinkSVCDest.Ref, ""),
					WithInitBrokerConditions),
				createChannel(withChannelReady, withChannelDeadLetterSink(sinkSVCDest)),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				makeTLSSecret(),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReadyWithDLS,
					WithDeadLeaderSink(sinkSVCDest.Ref, ""),
					WithBrokerAddressURI(brokerAddress),
					WithBrokerStatusDLSURI(dlsURI),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithBrokersAddresses([]duckv1.Addressable{
						{
							Name:    pointer.String("https"),
							URL:     httpsURL(brokerName, testNS),
							CACerts: pointer.String(testCaCerts),
						},
						{
							Name: pointer.String("http"),
							URL:  brokerAddress,
						},
					}),
				),
			}},
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.TransportEncryption: feature.Permissive,
			}),
		},
		{
			Name: "TLS strict",
			Key:  testKey,
			Objects: []runtime.Object{
				makeDLSServiceAsUnstructured(),
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithDeadLeaderSink(sinkSVCDest.Ref, ""),
					WithInitBrokerConditions),
				createChannel(withChannelReady, withChannelDeadLetterSink(sinkSVCDest)),
				imcConfigMap(),
				NewEndpoints(filterServiceName, systemNS,
					WithEndpointsLabels(FilterLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewEndpoints(ingressServiceName, systemNS,
					WithEndpointsLabels(IngressLabels()),
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				makeTLSSecret(),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.MTChannelBrokerClassValue),
					WithBrokerConfig(config()),
					WithBrokerReadyWithDLS,
					WithDeadLeaderSink(sinkSVCDest.Ref, ""),
					WithBrokerAddressURI(brokerAddress),
					WithBrokerStatusDLSURI(dlsURI),
					WithChannelAddressAnnotation(triggerChannelURL),
					WithChannelAPIVersionAnnotation(triggerChannelAPIVersion),
					WithChannelKindAnnotation(triggerChannelKind),
					WithChannelNameAnnotation(triggerChannelName),
					WithBrokersAddresses([]duckv1.Addressable{
						{
							Name:    pointer.String("https"),
							URL:     httpsURL(brokerName, testNS),
							CACerts: pointer.String(testCaCerts),
						},
					}),
					WithBrokerAddressHTTPS(duckv1.Addressable{
						Name:    pointer.String("https"),
						URL:     httpsURL(brokerName, testNS),
						CACerts: pointer.String(testCaCerts),
					}),
				),
			}},
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.TransportEncryption: feature.Strict,
			}),
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)

		cm, err := listers.GetConfigMapLister().ConfigMaps(testNS).Get("config-features")
		if err == nil {
			flags, err := feature.NewFlagsConfigFromConfigMap(cm)
			if err != nil {
				panic(err)
			}
			ctx = feature.ToContext(ctx, flags)
		}

		r := &Reconciler{
			eventingClientSet:  fakeeventingclient.Get(ctx),
			dynamicClientSet:   fakedynamicclient.Get(ctx),
			subscriptionLister: listers.GetSubscriptionLister(),
			endpointsLister:    listers.GetEndpointsLister(),
			configmapLister:    listers.GetConfigMapLister(),
			secretLister:       listers.GetSecretLister(),
			channelableTracker: duck.NewListableTrackerFromTracker(ctx, channelable.Get, tracker.New(func(types.NamespacedName) {}, 0)),
		}
		return broker.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetBrokerLister(),
			controller.GetEventRecorder(ctx),
			r, "MTChannelBasedBroker")

	},
		false,
		logger,
	))
}

func httpsURL(name string, ns string) *apis.URL {
	return &apis.URL{
		Scheme: "https",
		Host:   network.GetServiceHostname(ingressServiceName, systemNS),
		Path:   fmt.Sprintf("/%s/%s", ns, name),
	}
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

// unstructuredOption modifies *unstructured.Unstructured contents.
type unstructuredOption func(*unstructured.Unstructured)

func withChannelStatusAddress(url string) unstructuredOption {
	return func(channel *unstructured.Unstructured) {
		if err := unstructured.SetNestedField(channel.Object, url,
			"status", "address", "url"); err != nil {
			panic(err)
		}
	}
}

func withChannelStatusDeadLetterSinkURI(uri string) unstructuredOption {
	return func(channel *unstructured.Unstructured) {
		unstructured.SetNestedField(channel.Object, uri,
			"status", "deadLetterSinkURI")
	}
}

func withChannelDeadLetterSink(d duckv1.Destination) unstructuredOption {
	u := map[string]interface{}{}
	b, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(b, &u); err != nil {
		panic(err)
	}

	return func(channel *unstructured.Unstructured) {
		unstructured.SetNestedField(channel.Object, u,
			"spec", "delivery", "deadLetterSink")
	}
}

func withChannelReady(channel *unstructured.Unstructured) {
	withChannelStatusAddress(triggerChannelURL)(channel)
	withChannelStatusDeadLetterSinkURI(dlsURI.String())(channel)
}

func createChannel(opts ...unstructuredOption) *unstructured.Unstructured {

	channel := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              fmt.Sprintf("%s-kne-trigger", brokerName),
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
				"labels": map[string]interface{}{
					eventing.BrokerLabelKey:                 brokerName,
					"eventing.knative.dev/brokerEverything": "true",
				},
				"annotations": map[string]interface{}{
					"eventing.knative.dev/scope": "cluster",
				},
			},
		},
	}

	for _, f := range opts {
		f(channel)
	}

	return channel
}

func createChannelNoHostInUrl(namespace string) *unstructured.Unstructured {
	name := fmt.Sprintf("%s-kne-trigger", brokerName)
	labels := map[string]interface{}{
		eventing.BrokerLabelKey:                 brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
	annotations := map[string]interface{}{
		"eventing.knative.dev/scope": "cluster",
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
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": "http://",
				},
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

func makeChannelDLSRefNamePatch(refName string) clientgotesting.PatchActionImpl {
	return clientgotesting.PatchActionImpl{
		ActionImpl: clientgotesting.ActionImpl{
			Namespace: testNS,
		},
		Name:  fmt.Sprintf("%s-kne-trigger", brokerName),
		Patch: []byte(`[{"op":"replace","path":"/spec/delivery/deadLetterSink/ref/name","value":"` + refName + `"}]`),
	}
}

func makeChannelDeliveryRetryPatch(retries int) clientgotesting.PatchActionImpl {
	return clientgotesting.PatchActionImpl{
		ActionImpl: clientgotesting.ActionImpl{
			Namespace: testNS,
		},
		Name:  fmt.Sprintf("%s-kne-trigger", brokerName),
		Patch: []byte(`[{"op":"add","path":"/spec/delivery","value":{"retry":` + strconv.Itoa(retries) + `}}]`),
	}
}

const (
	testCaCerts = `
-----BEGIN CERTIFICATE-----
MIIDmjCCAoKgAwIBAgIUYzA4bTMXevuk3pl2Mn8hpCYL2C0wDQYJKoZIhvcNAQEL
BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
LUNBMB4XDTIzMDQwNTEzMTUyNFoXDTI2MDEyMzEzMTUyNFowbTELMAkGA1UEBhMC
VVMxEjAQBgNVBAgMCVlvdXJTdGF0ZTERMA8GA1UEBwwIWW91ckNpdHkxHTAbBgNV
BAoMFEV4YW1wbGUtQ2VydGlmaWNhdGVzMRgwFgYDVQQDDA9sb2NhbGhvc3QubG9j
YWwwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC5teo+En6U5nhqn7Sc
uanqswUmPlgs9j/8l21Rhb4T+ezlYKGQGhbJyFFMuiCE1Rjn8bpCwi7Nnv12Y2nz
FhEv2Jx0yL3Tqx0Q593myqKDq7326EtbO7wmDT0XD03twH5i9XZ0L0ihPWn1mjUy
WxhnHhoFpXrsnQECJorZY6aTrFbGVYelIaj5AriwiqyL0fET8pueI2GwLjgWHFSH
X8XsGAlcLUhkQG0Z+VO9usy4M1Wpt+cL6cnTiQ+sRmZ6uvaj8fKOT1Slk/oUeAi4
WqFkChGzGzLik0QrhKGTdw3uUvI1F2sdQj0GYzXaWqRz+tP9qnXdzk1GrszKKSlm
WBTLAgMBAAGjcDBuMB8GA1UdIwQYMBaAFJJcCftus4vj98N0zQQautsjEu82MAkG
A1UdEwQCMAAwCwYDVR0PBAQDAgTwMBQGA1UdEQQNMAuCCWxvY2FsaG9zdDAdBgNV
HQ4EFgQUnu/3vqA3VEzm128x/hLyZzR9JlgwDQYJKoZIhvcNAQELBQADggEBAFc+
1cKt/CNjHXUsirgEhry2Mm96R6Yxuq//mP2+SEjdab+FaXPZkjHx118u3PPX5uTh
gTT7rMfka6J5xzzQNqJbRMgNpdEFH1bbc11aYuhi0khOAe0cpQDtktyuDJQMMv3/
3wu6rLr6fmENo0gdcyUY9EiYrglWGtdXhlo4ySRY8UZkUScG2upvyOhHTxVCAjhP
efbMkNjmDuZOMK+wqanqr5YV6zMPzkQK7DspfRgasMAQmugQu7r2MZpXg8Ilhro1
s/wImGnMVk5RzpBVrq2VB9SkX/ThTVYEC/Sd9BQM364MCR+TA1l8/ptaLFLuwyw8
O2dgzikq8iSy1BlRsVw=
-----END CERTIFICATE-----
`
)

func makeTLSSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: systemNS,
			Name:      brokerIngressTLSSecretName,
		},
		Data: map[string][]byte{
			"ca.crt": []byte(testCaCerts),
		},
		Type: corev1.SecretTypeTLS,
	}
}
