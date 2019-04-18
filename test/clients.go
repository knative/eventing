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

// This file contains an object which encapsulates k8s clients which are useful for e2e tests.

package test

import (
	eventing "github.com/knative/eventing/pkg/client/clientset/versioned"
	"github.com/knative/pkg/test"
	"k8s.io/client-go/dynamic"
)

// Clients holds instances of interfaces for making requests to Knative.
type Clients struct {
	Kube     *test.KubeClient
	Eventing *eventing.Clientset
	Dynamic  dynamic.Interface
}

// NewClients instantiates and returns several clientsets required for making request to the
// cluster specified by the combination of clusterName and configPath.
func NewClients(configPath string, clusterName string) (*Clients, error) {
	clients := &Clients{}
	cfg, err := test.BuildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}
	clients.Kube, err = test.NewKubeClient(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	clients.Eventing, err = eventing.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clients.Dynamic, err = dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return clients, nil
}
