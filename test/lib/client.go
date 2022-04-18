/*
Copyright 2018 The Knative Authors

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

// This file contains an object which encapsulates k8s clients and other info which are useful for e2e tests.
// Each test case will need to create its own client.

package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/test"
	configtracing "knative.dev/pkg/tracing/config"

	eventing "knative.dev/eventing/pkg/client/clientset/versioned"
	"knative.dev/eventing/test/lib/duck"
)

// Client holds instances of interfaces for making requests to Knative.
type Client struct {
	Kube          kubernetes.Interface
	Eventing      *eventing.Clientset
	Apiextensions *apiextensionsv1.ApiextensionsV1Client
	Dynamic       dynamic.Interface
	Config        *rest.Config

	EventListener *EventListener

	Namespace string
	T         *testing.T
	Tracker   *Tracker

	podsCreated []string

	TracingCfg string
	loggingCfg string

	cleanup func()
}

// NewClient instantiates and returns several clientsets required for making request to the
// cluster specified by the combination of clusterName and configPath.
func NewClient(namespace string, t *testing.T) (*Client, error) {
	var err error

	client := &Client{}
	client.Config, err = test.Flags.GetRESTConfig()
	if err != nil {
		return nil, err
	}
	client.Kube, err = kubernetes.NewForConfig(client.Config)
	if err != nil {
		return nil, err
	}

	client.Eventing, err = eventing.NewForConfig(client.Config)
	if err != nil {
		return nil, err
	}

	client.Apiextensions, err = apiextensionsv1.NewForConfig(client.Config)
	if err != nil {
		return nil, err
	}

	client.Dynamic, err = dynamic.NewForConfig(client.Config)
	if err != nil {
		return nil, err
	}

	client.Namespace = namespace
	client.T = t
	client.Tracker = NewTracker(t, client.Dynamic)

	// Start informer
	client.EventListener = NewEventListener(client.Kube, client.Namespace, client.T.Logf)
	client.Cleanup(client.EventListener.Stop)

	client.TracingCfg, err = getTracingConfig(client.Kube)
	if err != nil {
		return nil, err
	}

	client.loggingCfg, err = getLoggingConfig(client.Kube)
	if err != nil {
		t.Log("Cannot retrieve the logging config map: ", err)
	}

	return client, nil
}

// Cleanup acts similarly to testing.T, but it's tied to the client lifecycle
func (c *Client) Cleanup(f func()) {
	oldCleanup := c.cleanup
	c.cleanup = func() {
		if oldCleanup != nil {
			defer oldCleanup()
		}
		f()
	}
}

func (c *Client) runCleanup() (err error) {
	if c.cleanup == nil {
		return nil
	}
	defer func() {
		if panicVal := recover(); panicVal != nil {
			err = fmt.Errorf("panic in cleanup function: %+v", panicVal)
		}
	}()

	c.cleanup()
	return nil
}

func (c *Client) dumpResources() {
	for _, metaResource := range c.Tracker.resourcesToCheckStatus {
		obj, err := duck.GetGenericObject(c.Dynamic, &metaResource, getGenericResource(metaResource.TypeMeta))
		if err != nil {
			c.T.Logf("Failed to get generic object %s/%s: %v", metaResource.GetNamespace(), metaResource.GetName(), err)
		}
		b, _ := json.MarshalIndent(obj, "", " ")
		c.T.Logf("Resource %s/%s %v:\n%s\n", metaResource.GetNamespace(), metaResource.GetName(), obj.GetObjectKind(), string(b))
	}
}

func getGenericResource(tm metav1.TypeMeta) runtime.Object {
	if tm.APIVersion == "v1" && tm.Kind == "Pod" {
		return &corev1.Pod{}
	}
	return &duckv1.KResource{}
}

func getTracingConfig(c kubernetes.Interface) (string, error) {
	cm, err := c.CoreV1().ConfigMaps(system.Namespace()).Get(context.Background(), configtracing.ConfigName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error while retrieving the %s config map: %+v", configtracing.ConfigName, errors.WithStack(err))
	}

	config, err := configtracing.NewTracingConfigFromConfigMap(cm)
	if err != nil {
		return "", fmt.Errorf("error while parsing the %s config map: %+v", configtracing.ConfigName, errors.WithStack(err))
	}

	configSerialized, err := configtracing.TracingConfigToJSON(config)
	if err != nil {
		return "", fmt.Errorf("error while serializing the %s config map: %+v", configtracing.ConfigName, errors.WithStack(err))
	}

	return configSerialized, nil
}

func getLoggingConfig(c kubernetes.Interface) (string, error) {
	cm, err := c.CoreV1().ConfigMaps(system.Namespace()).Get(context.Background(), logging.ConfigMapName(), metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error while retrieving the %s config map: %+v", logging.ConfigMapName(), errors.WithStack(err))
	}

	config, err := logging.NewConfigFromMap(cm.Data)
	if err != nil {
		return "", fmt.Errorf("error while parsing the %s config map: %+v", logging.ConfigMapName(), errors.WithStack(err))
	}

	configSerialized, err := logging.ConfigToJSON(config)
	if err != nil {
		return "", fmt.Errorf("error while serializing the %s config map: %+v", logging.ConfigMapName(), errors.WithStack(err))
	}

	return configSerialized, nil
}
