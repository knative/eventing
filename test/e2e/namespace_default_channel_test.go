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
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"knative.dev/pkg/system"

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

		defaults := make(map[string]interface{})
		err = yaml.Unmarshal([]byte(cm.Data["default-ch-config"]), defaults)
		assert.Nil(t, err)

		defaults["namespaceDefaults"] = map[string]interface{}{
			c.Namespace: map[string]interface{}{
				"apiVersion": "messaging.knative.dev/v1",
				"kind":       "InMemoryChannel",
				"spec": map[string]interface{}{
					"delivery": map[string]interface{}{
						"retry":         5,
						"backoffPolicy": "exponential",
						"backoffDelay":  "PT0.5S",
					},
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

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1",
			"kind":       "Channel",
			"metadata": map[string]interface{}{
				"name":      "xyz",
				"namespace": c.Namespace,
			},
		},
	}

	_, err = c.Dynamic.
		Resource(schema.GroupVersionResource{Group: "messaging.knative.dev", Version: "v1", Resource: "channels"}).
		Namespace(c.Namespace).
		Create(ctx, obj, metav1.CreateOptions{})

	assert.Nil(t, err)
}
