// +build e2e

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

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	"knative.dev/eventing/pkg/apis/config"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

func TestBrokerNamespaceDefaulting(t *testing.T) {
	ctx := context.Background()

	c := testlib.Setup(t, true)
	defer testlib.TearDown(c)

	err := reconciler.RetryTestErrors(func(attempt int) error {

		t.Log("Updating defaulting ConfigMap attempt:", attempt)

		cm, err := c.Kube.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.DefaultsConfigName, metav1.GetOptions{})
		assert.Nil(t, err)

		// Preserve existing namespace defaults.
		defaults := make(map[string]map[string]interface{})
		err = yaml.Unmarshal([]byte(cm.Data[config.BrokerDefaultsKey]), &defaults)
		assert.Nil(t, err)

		if _, ok := defaults["namespaceDefaults"]; !ok {
			defaults["namespaceDefaults"] = make(map[string]interface{})
		}

		defaults["namespaceDefaults"][c.Namespace] = map[string]interface{}{
			"apiVersion":  "v1",
			"kind":        "ConfigMap",
			"name":        "config-br-default-channel",
			"namespace":   "knative-eventing",
			"brokerClass": brokerClass,
			"delivery": map[string]interface{}{
				"retry":         5,
				"backoffPolicy": "exponential",
				"backoffDelay":  "PT0.5S",
			},
		}

		b, err := yaml.Marshal(defaults)
		assert.Nil(t, err)

		cm.Data[config.BrokerDefaultsKey] = string(b)

		cm, err = c.Kube.CoreV1().ConfigMaps(system.Namespace()).Update(ctx, cm, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		b, err = yaml.Marshal(cm.Data[config.BrokerDefaultsKey])
		if err != nil {
			t.Log("error", err)
		} else {
			t.Log("CM updated - new values:", string(b))
		}

		return nil
	})
	assert.Nil(t, err)

	// Create a Broker and check whether it has the DeliverySpec set as we specified above.
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

		name := fmt.Sprintf("%s-%d", namePrefix, n)
		lastName = name

		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "eventing.knative.dev/v1",
				"kind":       "Broker",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": c.Namespace,
				},
			},
		}

		createdObj, err := c.Dynamic.
			Resource(schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1", Resource: "brokers"}).
			Namespace(c.Namespace).
			Create(ctx, obj, metav1.CreateOptions{})
		assert.Nil(t, err)
		n = n + 1

		broker := &eventingv1.Broker{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(createdObj.Object, broker)
		assert.Nil(t, err)

		if !webhookObservedBrokerUpdate(broker) {
			return fmt.Errorf("webhook hasn't seen the update: %+v", broker)
		}

		assert.Equal(t, brokerClass, broker.Annotations[eventingv1.BrokerClassAnnotationKey])

		if err != nil || !webhookObservedBrokerUpdateFromDeliverySpec(broker.Spec.Delivery) {
			return fmt.Errorf("webhook hasn't seen the update: %+v", broker.Spec.Delivery)
		}

		assert.Equal(t, "PT0.5S", *broker.Spec.Delivery.BackoffDelay)
		assert.Equal(t, int32(5), *broker.Spec.Delivery.Retry)
		assert.Equal(t, eventingduck.BackoffPolicyExponential, *broker.Spec.Delivery.BackoffPolicy)

		return nil
	})
	assert.Nil(t, err)

	err = wait.Poll(time.Second, time.Minute, func() (done bool, err error) {
		foundBroker, err := c.Eventing.EventingV1().Brokers(c.Namespace).Get(ctx, lastName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		assert.Nil(t, err)

		assert.Equal(t, brokerClass, foundBroker.Annotations[eventingv1.BrokerClassAnnotationKey])
		assert.Equal(t, "PT0.5S", *foundBroker.Spec.Delivery.BackoffDelay)
		assert.Equal(t, int32(5), *foundBroker.Spec.Delivery.Retry)
		assert.Equal(t, eventingduck.BackoffPolicyExponential, *foundBroker.Spec.Delivery.BackoffPolicy)

		return true, nil
	})
	assert.Nil(t, err)
}

func webhookObservedBrokerUpdate(br *eventingv1.Broker) bool {
	_, ok := br.Annotations[eventingv1.BrokerClassAnnotationKey]
	return ok
}

func webhookObservedBrokerUpdateFromDeliverySpec(d *eventingduck.DeliverySpec) bool {
	return d != nil && d.BackoffDelay != nil && d.Retry != nil && d.BackoffPolicy != nil
}
