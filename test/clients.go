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
	channelstyped "github.com/knative/eventing/pkg/client/clientset/versioned/typed/channels/v1alpha1"
	feedstyped "github.com/knative/eventing/pkg/client/clientset/versioned/typed/feeds/v1alpha1"
	flowstyped "github.com/knative/eventing/pkg/client/clientset/versioned/typed/flows/v1alpha1"
	"github.com/knative/pkg/test"
	serving "github.com/knative/serving/pkg/client/clientset/versioned"
	servingtyped "github.com/knative/serving/pkg/client/clientset/versioned/typed/serving/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coretyped "k8s.io/client-go/kubernetes/typed/core/v1"
	rbactyped "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Clients holds instances of interfaces for making requests to Knative.
type Clients struct {
	Kube     *test.KubeClient
	Serving  *serving.Clientset
	Eventing *eventing.Clientset
}

// NewClients instantiates and returns several clientsets required for making request to the
// cluster specified by the combination of clusterName and configPath. Clients can
// make requests within namespace.
func NewClients(configPath string, clusterName string, namespace string) (*Clients, error) {
	clients := &Clients{}
	cfg, err := buildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}
	clients.Kube, err = test.NewKubeClient(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	clients.Serving, err = serving.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clients.Eventing, err = eventing.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return clients, nil
}

// DeleterInterface has the method that is called in order to delete a resource
type DeleterInterface interface {
	Delete(name string, options *metav1.DeleteOptions) error
}

// Delete will delete all resources and wait until they're deleted
func (clients *Clients) Delete(resource interface{}, name string) error {
	if res, ok := resource.(DeleterInterface); ok {
		if err := res.Delete(name, nil); err != nil {
			return err
		}
	}

	if res, ok := resource.(coretyped.PodInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(coretyped.ServiceAccountInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(rbactyped.ClusterRoleBindingInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(servingtyped.RouteInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(servingtyped.ConfigurationInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(flowstyped.FlowInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(channelstyped.ClusterBusInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(channelstyped.ChannelInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(feedstyped.EventSourceInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(feedstyped.EventTypeInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}

	if res, ok := resource.(channelstyped.SubscriptionInterface); ok {
		return wait.PollImmediate(interval, timeout, func() (bool, error) {
			if _, err := res.Get(name, metav1.GetOptions{}); err != nil {
				return true, nil
			}
			return false, nil
		})
	}
	return nil
}

func buildClientConfig(kubeConfigPath string, clusterName string) (*rest.Config, error) {
	overrides := clientcmd.ConfigOverrides{}
	// Override the cluster name if provided.
	if clusterName != "" {
		overrides.Context.Cluster = clusterName
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath},
		&overrides).ClientConfig()
}
