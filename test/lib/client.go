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
	"fmt"
	"testing"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/test"
	configtracing "knative.dev/pkg/tracing/config"

	eventing "knative.dev/eventing/pkg/client/clientset/versioned"
	ti "knative.dev/eventing/test/test_images"
)

// Client holds instances of interfaces for making requests to Knative.
type Client struct {
	Kube          *test.KubeClient
	Eventing      *eventing.Clientset
	Apiextensions *apiextensionsv1beta1.ApiextensionsV1beta1Client
	Dynamic       dynamic.Interface
	Config        *rest.Config

	EventListener *EventListener

	Namespace string
	T         *testing.T
	Tracker   *Tracker

	podsCreated []string

	tracingEnv corev1.EnvVar
	loggingEnv *corev1.EnvVar

	cleanup func()
}

// NewClient instantiates and returns several clientsets required for making request to the
// cluster specified by the combination of clusterName and configPath.
func NewClient(configPath string, clusterName string, namespace string, t *testing.T) (*Client, error) {
	var err error

	client := &Client{}
	client.Config, err = test.BuildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}
	client.Kube, err = test.NewKubeClient(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	client.Eventing, err = eventing.NewForConfig(client.Config)
	if err != nil {
		return nil, err
	}

	client.Apiextensions, err = apiextensionsv1beta1.NewForConfig(client.Config)
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

	client.tracingEnv, err = getTracingConfig(client.Kube)
	if err != nil {
		return nil, err
	}

	client.loggingEnv, err = getLoggingConfig(client.Kube)
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

func getTracingConfig(c kubernetes.Interface) (corev1.EnvVar, error) {
	cm, err := c.CoreV1().ConfigMaps(system.Namespace()).Get(context.Background(), configtracing.ConfigName, metav1.GetOptions{})
	if err != nil {
		return corev1.EnvVar{}, fmt.Errorf("error while retrieving the %s config map: %+v", configtracing.ConfigName, errors.WithStack(err))
	}

	config, err := configtracing.NewTracingConfigFromConfigMap(cm)
	if err != nil {
		return corev1.EnvVar{}, fmt.Errorf("error while parsing the %s config map: %+v", configtracing.ConfigName, errors.WithStack(err))
	}

	configSerialized, err := configtracing.TracingConfigToJSON(config)
	if err != nil {
		return corev1.EnvVar{}, fmt.Errorf("error while serializing the %s config map: %+v", configtracing.ConfigName, errors.WithStack(err))
	}

	return corev1.EnvVar{Name: ti.ConfigTracingEnv, Value: configSerialized}, nil
}

func getLoggingConfig(c kubernetes.Interface) (*corev1.EnvVar, error) {
	cm, err := c.CoreV1().ConfigMaps(system.Namespace()).Get(context.Background(), logging.ConfigMapName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error while retrieving the %s config map: %+v", logging.ConfigMapName(), errors.WithStack(err))
	}

	config, err := logging.NewConfigFromMap(cm.Data)
	if err != nil {
		return nil, fmt.Errorf("error while parsing the %s config map: %+v", logging.ConfigMapName(), errors.WithStack(err))
	}

	configSerialized, err := logging.ConfigToJSON(config)
	if err != nil {
		return nil, fmt.Errorf("error while serializing the %s config map: %+v", logging.ConfigMapName(), errors.WithStack(err))
	}

	return &corev1.EnvVar{Name: ti.ConfigLoggingEnv, Value: configSerialized}, nil
}
