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
	"testing"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/test"

	eventing "knative.dev/eventing/pkg/client/clientset/versioned"
)

// Client holds instances of interfaces for making requests to Knative.
type Client struct {
	Kube     *test.KubeClient
	Eventing *eventing.Clientset
	Dynamic  dynamic.Interface
	Config   *rest.Config

	Namespace string
	T         *testing.T
	Tracker   *Tracker

	podsCreated []string
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

	client.Dynamic, err = dynamic.NewForConfig(client.Config)
	if err != nil {
		return nil, err
	}

	client.Namespace = namespace
	client.T = t
	client.Tracker = NewTracker(t, client.Dynamic)
	return client, nil
}
