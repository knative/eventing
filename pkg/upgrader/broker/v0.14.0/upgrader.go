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

package broker

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/pkg/apis/duck"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

// Upgrade upgrades all the brokers by applying the Broker Class to all the
// ones that do not have them. This is necessary to ensure that existing Brokers
// that do not have the eventing.knative.dev/broker.class will continue to be
// reconciled by the existing ChannelBasedBroker
func Upgrade(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	nsClient := kubeclient.Get(ctx).CoreV1().Namespaces()
	namespaces, err := nsClient.List(metav1.ListOptions{})
	if err != nil {
		logger.Warnf("Failed to list namespaces: %v", err)
		return err
	}
	for _, ns := range namespaces.Items {
		err = processNamespace(ctx, ns.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func processNamespace(ctx context.Context, ns string) error {
	logger := logging.FromContext(ctx)
	logger.Infof("Processing Brokers in namespace: %q", ns)

	eventingClient := eventingclient.Get(ctx)
	brokerClient := eventingClient.EventingV1alpha1().Brokers(ns)
	brokers, err := brokerClient.List(metav1.ListOptions{})
	if err != nil {
		logger.Warnf("Failed to list brokers for namespace %q: %v", ns, err)
		return err
	}
	for _, broker := range brokers.Items {
		patch, err := processBroker(ctx, broker)
		if err != nil {
			logger.Warnf("Failed to process a Broker \"%s/%s\" : %v", broker.Namespace, broker.Name, err)
			return err
		}
		if len(patch) == 0 {
			logger.Infof("Broker \"%s/%s\" has annotation already", broker.Namespace, broker.Name)
			continue
		}

		// Ok, there are differences, apply the patch
		logger.Infof("Patching Broker \"%s/%s\" with %q", broker.Namespace, broker.Name, string(patch))
		patched, err := brokerClient.Patch(broker.Name, types.MergePatchType, patch)
		if err != nil {
			logger.Warnf("Failed to patch \"%s/%s\" : %v", broker.Namespace, broker.Name, err)
			return err
		}
		logger.Infof("Patched \"%s/%s\" successfully new Annotations: %+v", broker.Namespace, broker.Name, patched.ObjectMeta.GetAnnotations())
	}
	return nil
}

// Process a single Broker to see if it needs a patch applied to it or not.
// Returns non-empty patch bytes if a patch is necessary.
func processBroker(ctx context.Context, broker v1alpha1.Broker) ([]byte, error) {
	logger := logging.FromContext(ctx)

	modified := broker.DeepCopy()
	annotations := modified.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	if brokerClass, present := annotations[eventing.BrokerClassKey]; present {
		logger.Infof("Annotation found \"%s/%s\" => %q", broker.Namespace, broker.Name, brokerClass)
		return []byte{}, nil
	}
	if _, present := annotations[eventing.BrokerClassKey]; !present {
		annotations[eventing.BrokerClassKey] = eventing.ChannelBrokerClassValue
		modified.ObjectMeta.SetAnnotations(annotations)
	}
	patch, err := duck.CreateMergePatch(broker, modified)
	if err != nil {
		logger.Warnf("Failed to create patch for \"%s/%s\" : %v", broker.Namespace, broker.Name, err)
		return []byte{}, err
	}
	logger.Infof("Patched \"%s/%s\": %q", broker.Namespace, broker.Name, string(patch))
	// If there is nothing to patch, we are good, just return.
	// Empty patch is {}, hence we check for that.
	if len(patch) <= 2 {
		return []byte{}, nil
	}
	return patch, nil
}
