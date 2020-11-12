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

package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"knative.dev/pkg/system"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	"github.com/ghodss/yaml"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	reconciler "knative.dev/pkg/reconciler"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/config"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

const (
	// configMapName is the name of the ConfigMap that contains the configuration for the default
	// channel CRD.
	configMapName = config.ChannelDefaultsConfigName

	// channelDefaulterKey is the key in the ConfigMap to get the name of the default
	// Channel CRD.
	channelDefaulterKey = config.ChannelDefaulterKey
)

// ChannelClusterDefaulterTestHelper is the helper function for channel_defaulter_test
func ChannelClusterDefaulterTestHelper(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		// these tests cannot be run in parallel as they have cluster-wide impact
		client := testlib.Setup(st, false, options...)
		defer testlib.TearDown(client)

		if err := updateDefaultChannelCM(client, func(conf *config.ChannelDefaults) {
			setClusterDefaultChannel(conf, channel)
		}); err != nil {
			st.Fatal("Failed to update the defaultchannel configmap:", err)
		}

		defaultChannelTestHelper(ctx, st, client, channel)
	})
}

// ChannelNamespaceDefaulterTestHelper is the helper function for channel_defaulter_test
func ChannelNamespaceDefaulterTestHelper(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		// we cannot run these tests in parallel as the updateDefaultChannelCM function is not thread-safe
		// TODO(chizhg): make updateDefaultChannelCM thread-safe and run in parallel if the tests are taking too long to finish
		client := testlib.Setup(st, false, options...)
		defer testlib.TearDown(client)

		if err := updateDefaultChannelCM(client, func(conf *config.ChannelDefaults) {
			setNamespaceDefaultChannel(conf, client.Namespace, channel)
		}); err != nil {
			st.Fatal("Failed to update the defaultchannel configmap:", err)
		}

		defaultChannelTestHelper(ctx, st, client, channel)
	})
}

func defaultChannelTestHelper(ctx context.Context, t *testing.T, client *testlib.Client, expectedChannel metav1.TypeMeta) {
	channelName := "e2e-defaulter-channel"
	senderName := "e2e-defaulter-sender"
	subscriptionName := "e2e-defaulter-subscription"
	recordEventsPodName := "e2e-defaulter-recordevents-pod"

	// create channel
	client.CreateChannelWithDefaultOrFail(eventingtesting.NewChannel(channelName, client.Namespace))

	// create event logger pod and service as the subscriber
	eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)
	// create subscription to subscribe the channel, and forward the received events to the logger service
	client.CreateSubscriptionOrFail(
		subscriptionName,
		channelName,
		testlib.ChannelTypeMeta,
		resources.WithSubscriberForSubscription(recordEventsPodName),
	)

	// wait for all test resources to be ready, so that we can start sending events
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// check if the defaultchannel creates exactly one underlying channel given the spec
	metaResourceList := resources.NewMetaResourceList(client.Namespace, &expectedChannel)
	objs, err := duck.GetGenericObjectList(client.Dynamic, metaResourceList, &eventingduck.SubscribableType{})
	if err != nil {
		t.Fatal("Failed to list the underlying channels:", err)
	}

	// Note that since by default MT ChannelBroker creates a Broker in each namespace, there's
	// actually two channels.
	// https://github.com/knative/eventing/issues/3138
	// So, filter out the broker channel from the list before checking that there's only one.
	filteredObjs := make([]runtime.Object, 0)
	for _, o := range objs {
		if o.(*eventingduck.SubscribableType).Name != "default-kne-trigger" {
			filteredObjs = append(filteredObjs, o)
		}
	}

	if len(filteredObjs) != 1 {
		t.Logf("Got unexpected channels:")
		for i, ec := range filteredObjs {
			t.Logf("Extra channels: %d : %+v", i, ec)
		}
		t.Fatal("The defaultchannel is expected to create 1 underlying channel, but got", len(filteredObjs))
	}

	// send CloudEvent to the channel
	event := cloudevents.NewEvent()
	event.SetID("test")
	eventSource := fmt.Sprintf("http://%s.svc/", senderName)
	event.SetSource(eventSource)
	event.SetType(testlib.DefaultEventType)
	body := fmt.Sprintf(`{"msg":"TestSingleEvent %s"}`, uuid.New().String())
	if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
		t.Fatal("Cannot set the payload of the event:", err.Error())
	}
	client.SendEventToAddressable(ctx, senderName, channelName, testlib.ChannelTypeMeta, event)

	// verify the logger service receives the event
	eventTracker.AssertAtLeast(1, recordevents.MatchEvent(
		HasSource(eventSource),
		HasData([]byte(body)),
	))
}

// updateDefaultChannelCM will update the default channel configmap
func updateDefaultChannelCM(client *testlib.Client, updateConfig func(config *config.ChannelDefaults)) error {
	cmInterface := client.Kube.CoreV1().ConfigMaps(system.Namespace())

	err := reconciler.RetryUpdateConflicts(func(attempts int) (err error) {
		// get the defaultchannel configmap
		configMap, err := cmInterface.Get(context.Background(), configMapName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		// get the defaultchannel config value
		defaultChannelConfig, hasDefault := configMap.Data[channelDefaulterKey]
		config := &config.ChannelDefaults{}
		if hasDefault {
			if err := yaml.Unmarshal([]byte(defaultChannelConfig), config); err != nil {
				return err
			}
		}

		// update the defaultchannel config
		updateConfig(config)
		configBytes, err := yaml.Marshal(*config)
		if err != nil {
			return err
		}
		// update the defaultchannel configmap
		configMap.Data[channelDefaulterKey] = string(configBytes)
		_, err = cmInterface.Update(context.Background(), configMap, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return err
	}
	// In cmd/webhook.go, configMapWatcher watches the configmap changes and set the config for channeldefaulter,
	// the resync time is set to 0, which means the the resync will be delayed as long as possible (until the upstream
	// source closes the watch or times out, or you stop the controller)
	// Wait for 5 seconds to let the ConfigMap be synced up.
	// TODO(chizhg): 5 seconds is an empirical duration, and does not solve the problem from the root.
	// To make it work reliably, we may need to manually restart the controller.
	// https://github.com/knative/eventing/issues/2807
	time.Sleep(5 * time.Second)
	return nil
}

// setClusterDefaultChannel will set the default channel for cluster-wide
func setClusterDefaultChannel(cfg *config.ChannelDefaults, channel metav1.TypeMeta) {
	if cfg.ClusterDefault == nil {
		cfg.ClusterDefault = &config.ChannelTemplateSpec{}
	}
	// If we're testing with Channel, we can't default to ourselves, or badness will
	// happen. We're going to try to create Channels that are ourselves.
	if channel.Kind == "Channel" {
		channel.Kind = "InMemoryChannel"
	}
	cfg.ClusterDefault.TypeMeta = channel
}

// setNamespaceDefaultChannel will set the default channel for namespace-wide
func setNamespaceDefaultChannel(cfg *config.ChannelDefaults, namespace string, channel metav1.TypeMeta) {
	// If we're testing with Channel, we can't default to ourselves, or badness will
	// happen. We're going to try to create Channels that are ourselves.
	if channel.Kind == "Channel" {
		channel.Kind = "InMemoryChannel"
	}
	if cfg.NamespaceDefaults == nil {
		cfg.NamespaceDefaults = make(map[string]*config.ChannelTemplateSpec, 1)
	}
	namespaceDefaults := cfg.NamespaceDefaults
	if spec, exists := namespaceDefaults[namespace]; exists {
		spec.TypeMeta = channel
	} else {
		spec = &config.ChannelTemplateSpec{
			TypeMeta: channel,
		}
		namespaceDefaults[namespace] = spec
	}
}
