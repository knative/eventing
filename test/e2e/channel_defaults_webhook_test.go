// +build e2e

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

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"knative.dev/pkg/system"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	testlib "knative.dev/eventing/test/lib"
)

func TestChannelNamespaceDefaulting(t *testing.T) {

	ctx := context.Background()

	const (
		defaultChannelCM        = "default-ch-webhook"
		defaultChannelConfigKey = "default-ch-config"
	)

	c := testlib.Setup(t, true)
	defer testlib.TearDown(c)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {

		t.Log("Updating defaulting ConfigMap")

		cm, err := c.Kube.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, defaultChannelCM, metav1.GetOptions{})
		assert.Nil(t, err)

		// Preserve existing namespace defaults.
		defaults := make(map[string]map[string]interface{})
		err = yaml.Unmarshal([]byte(cm.Data[defaultChannelConfigKey]), defaults)
		assert.Nil(t, err)

		defaults["namespaceDefaults"][c.Namespace] = map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1",
			"kind":       "InMemoryChannel",
			"spec": map[string]interface{}{
				"delivery": map[string]interface{}{
					"retry":         5,
					"backoffPolicy": "exponential",
					"backoffDelay":  "PT0.5S",
				},
			},
		}

		b, err := yaml.Marshal(defaults)
		assert.Nil(t, err)

		cm.Data[defaultChannelConfigKey] = string(b)

		cm, err = c.Kube.CoreV1().ConfigMaps(system.Namespace()).Update(ctx, cm, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		b, err = yaml.Marshal(cm.Data[defaultChannelConfigKey])
		if err != nil {
			t.Log("error", err)
		} else {
			t.Log("CM updated - new values:", string(b))
		}

		return nil
	})
	assert.Nil(t, err)

	// Create a Channel and check whether it has the DeliverySpec set as we specified above.
	// Since the webhook receives the updates at some undetermined time after the update to reduce flakiness retry after
	// a delay and check whether it has the shape we want it to have.

	namePrefix := "xyz"
	n := 0
	lastName := ""

	backoff := wait.Backoff{
		Duration: time.Second,
		Factor:   1.0,
		Jitter:   0.1,
		Steps:    5,
	}

	err = retry.OnError(backoff, func(err error) bool { return err != nil }, func() error {
		n++
		name := fmt.Sprintf("%s-%d", namePrefix, n)
		lastName = name

		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "messaging.knative.dev/v1",
				"kind":       "Channel",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": c.Namespace,
				},
			},
		}

		createdObj, err := c.Dynamic.
			Resource(schema.GroupVersionResource{Group: "messaging.knative.dev", Version: "v1", Resource: "channels"}).
			Namespace(c.Namespace).
			Create(ctx, obj, metav1.CreateOptions{})
		assert.Nil(t, err)

		channel := &messagingv1.Channel{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(createdObj.Object, channel)
		assert.Nil(t, err)

		if !webhookObservedUpdate(channel) {
			return fmt.Errorf("webhook hasn't seen the update: %+v", channel)
		}

		delivery := struct {
			Delivery eventingduck.DeliverySpec `json:"delivery"`
		}{}
		err = json.Unmarshal(channel.Spec.ChannelTemplate.Spec.Raw, &delivery)
		if err != nil || !webhookObservedUpdateFromDeliverySpec(&delivery.Delivery) {
			return fmt.Errorf("webhook hasn't seen the update: %+v %+v", delivery, string(channel.Spec.ChannelTemplate.Spec.Raw))
		}

		assert.Equal(t, "PT0.5S", *delivery.Delivery.BackoffDelay)
		assert.Equal(t, int32(5), *delivery.Delivery.Retry)
		assert.Equal(t, eventingduck.BackoffPolicyExponential, *delivery.Delivery.BackoffPolicy)

		return nil
	})
	assert.Nil(t, err)

	err = wait.Poll(time.Second, time.Minute, func() (done bool, err error) {
		imc, err := c.Eventing.MessagingV1().InMemoryChannels(c.Namespace).Get(ctx, lastName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		assert.Nil(t, err)

		assert.Equal(t, "PT0.5S", *imc.Spec.Delivery.BackoffDelay)
		assert.Equal(t, int32(5), *imc.Spec.Delivery.Retry)
		assert.Equal(t, eventingduck.BackoffPolicyExponential, *imc.Spec.Delivery.BackoffPolicy)

		return true, nil
	})
	assert.Nil(t, err)
}

func webhookObservedUpdate(ch *messagingv1.Channel) bool {
	return ch.Spec.ChannelTemplate != nil &&
		ch.Spec.ChannelTemplate.Spec != nil
}

func webhookObservedUpdateFromDeliverySpec(d *eventingduck.DeliverySpec) bool {
	return d.BackoffDelay != nil && d.Retry != nil && d.BackoffPolicy != nil
}
