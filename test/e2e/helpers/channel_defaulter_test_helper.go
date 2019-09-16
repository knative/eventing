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
	"fmt"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/defaultchannel"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/base"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
)

const (
	// configMapName is the name of the ConfigMap that contains the configuration for the default
	// channel CRD.
	configMapName = defaultchannel.ConfigMapName

	// channelDefaulterKey is the key in the ConfigMap to get the name of the default
	// Channel CRD.
	channelDefaulterKey = defaultchannel.ChannelDefaulterKey
)

// ChannelClusterDefaulterTestHelper is the helper function for channel_defaulter_test
func ChannelClusterDefaulterTestHelper(t *testing.T, channelTestRunner common.ChannelTestRunner) {
	channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel string) {
		// these tests cannot be run in parallel as they have cluster-wide impact
		client := common.Setup(st, false)
		defer common.TearDown(client)

		if err := updateDefaultChannelCM(client, func(conf *defaultchannel.Config) {
			setClusterDefaultChannel(conf, channel)
		}); err != nil {
			st.Fatalf("Failed to update the defaultchannel configmap: %v", err)
		}

		defaultChannelTestHelper(st, client, channel)
	})
}

// ChannelNamespaceDefaulterTestHelper is the helper function for channel_defaulter_test
func ChannelNamespaceDefaulterTestHelper(t *testing.T, channelTestRunner common.ChannelTestRunner) {
	channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel string) {
		// we cannot run these tests in parallel as the updateDefaultChannelCM function is not thread-safe
		// TODO(Fredy-Z): make updateDefaultChannelCM thread-safe and run in parallel if the tests are taking too long to finish
		client := common.Setup(st, false)
		defer common.TearDown(client)

		if err := updateDefaultChannelCM(client, func(conf *defaultchannel.Config) {
			setNamespaceDefaultChannel(conf, client.Namespace, channel)
		}); err != nil {
			st.Fatalf("Failed to update the defaultchannel configmap: %v", err)
		}

		defaultChannelTestHelper(st, client, channel)
	})
}

func defaultChannelTestHelper(t *testing.T, client *common.Client, expectedChannelKind string) {
	channelName := "e2e-defaulter-channel"
	senderName := "e2e-defaulter-sender"
	subscriptionName := "e2e-defaulter-subscription"
	loggerPodName := "e2e-defaulter-logger-pod"

	// create channel
	client.CreateChannelWithDefaultOrFail(eventingtesting.NewChannel(channelName, client.Namespace))

	// create logger service as the subscriber
	pod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(pod, common.WithService(loggerPodName))

	// create subscription to subscribe the channel, and forward the received events to the logger service
	client.CreateSubscriptionOrFail(
		subscriptionName,
		channelName,
		common.ChannelTypeMeta,
		resources.WithSubscriberForSubscription(loggerPodName),
	)

	// wait for all test resources to be ready, so that we can start sending events
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// check if the defaultchannel creates exactly one underlying channel given the spec
	metaResourceList := resources.NewMetaResourceList(client.Namespace, common.GetChannelTypeMeta(expectedChannelKind))
	objs, err := base.GetGenericObjectList(client.Dynamic, metaResourceList, &eventingduck.SubscribableType{})
	if err != nil {
		t.Fatalf("Failed to list the underlying channels: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("The defaultchannel is expected to create 1 underlying channel, but got %d", len(objs))
	}

	// send fake CloudEvent to the channel
	body := fmt.Sprintf("TestSingleEvent %s", uuid.NewUUID())
	event := &resources.CloudEvent{
		Source:   senderName,
		Type:     resources.CloudEventDefaultType,
		Data:     fmt.Sprintf(`{"msg":%q}`, body),
		Encoding: resources.CloudEventDefaultEncoding,
	}

	if err := client.SendFakeEventToAddressable(senderName, channelName, common.ChannelTypeMeta, event); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the channel %q", channelName)
	}

	// verify the logger service receives the event
	if err := client.CheckLog(loggerPodName, common.CheckerContains(body)); err != nil {
		t.Fatalf("String %q not found in logs of logger pod %q: %v", body, loggerPodName, err)
	}
}

// updateDefaultChannelCM will update the default channel configmap
func updateDefaultChannelCM(client *common.Client, updateConfig func(config *defaultchannel.Config)) error {
	systemNamespace := resources.SystemNamespace
	cmInterface := client.Kube.Kube.CoreV1().ConfigMaps(systemNamespace)
	// get the defaultchannel configmap
	configMap, err := cmInterface.Get(configMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// get the defaultchannel config value
	defaultChannelConfig, hasDefault := configMap.Data[channelDefaulterKey]
	config := &defaultchannel.Config{}
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
	_, err = cmInterface.Update(configMap)
	// In cmd/webhook.go, configMapWatcher watches the configmap changes and set the config for channeldefaulter,
	// the resync time is set to 0, so 5 seconds should be enough to get the OnChange callback triggered.
	time.Sleep(5 * time.Second)
	return nil
}

// setClusterDefaultChannel will set the default channel for cluster-wide
func setClusterDefaultChannel(config *defaultchannel.Config, channel string) {
	config.ClusterDefault.TypeMeta = *common.GetChannelTypeMeta(channel)
}

// setNamespaceDefaultChannel will set the default channel for namespace-wide
func setNamespaceDefaultChannel(config *defaultchannel.Config, namespace, channel string) {
	namespaceDefaults := config.NamespaceDefaults
	if spec, exists := namespaceDefaults[namespace]; exists {
		spec.TypeMeta = *common.GetChannelTypeMeta(channel)
	} else {
		spec = &eventingduck.ChannelTemplateSpec{
			TypeMeta: *common.GetChannelTypeMeta(channel),
		}
		namespaceDefaults[namespace] = spec
	}
}
