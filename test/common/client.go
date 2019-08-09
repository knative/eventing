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

package common

import (
	"testing"

	"k8s.io/client-go/dynamic"
	eventing "knative.dev/eventing/pkg/client/clientset/versioned"
	"knative.dev/pkg/test"
)

// Client holds instances of interfaces for making requests to Knative.
type Client struct {
	Kube     *test.KubeClient
	Eventing *eventing.Clientset
	Dynamic  dynamic.Interface

	Namespace string
	T         *testing.T
	Tracker   *Tracker
}

// NewClient instantiates and returns several clientsets required for making request to the
// cluster specified by the combination of clusterName and configPath.
func NewClient(configPath string, clusterName string, namespace string, t *testing.T) (*Client, error) {
	client := &Client{}
	cfg, err := test.BuildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}
	client.Kube, err = test.NewKubeClient(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	client.Eventing, err = eventing.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	client.Dynamic, err = dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	client.Namespace = namespace
	client.T = t
	client.Tracker = NewTracker(t.Logf, client.Dynamic)
	return client, nil
}
