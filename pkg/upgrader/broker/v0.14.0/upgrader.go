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
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/pkg/apis/duck"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

// Upgrade upgrades all the brokers by applying an Annotation to all the
// ones that do not have them. This is necessary to ensure that existing Brokers
// that do not have Annotation will continue to be reconciled by the existing
// ChannelBasedBroker
func Upgrade(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	kubeClient := kubeclient.Get(ctx)
	eventingClient := eventingclient.Get(ctx)
	nsClient := kubeClient.CoreV1().Namespaces()
	namespaces, err := nsClient.List(metav1.ListOptions{})
	if err != nil {
		logger.Warnf("Failed to list namespaces: %v", err)
		return err
	}
	for _, ns := range namespaces.Items {
		logger.Infof("Processing Brokers in namespace: %q", ns.Name)
		brokerClient := eventingClient.EventingV1alpha1().Brokers(ns.Name)
		brokers, err := brokerClient.List(metav1.ListOptions{})
		if err != nil {
			logger.Warnf("Failed to list brokers for namespace %q: %v", ns.Name, err)
			return err
		}
		for _, broker := range brokers.Items {
			logger.Infof("Processing Broker \"%s/%s\"", broker.Namespace, broker.Name)

			annotations := broker.ObjectMeta.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string, 1)
			}
			if brokerClass, present := annotations[eventing.BrokerClassKey]; present {
				logger.Infof("Annotation found \"%s/%s\" => %q", broker.Namespace, broker.Name, brokerClass)
				continue
			}

			// No annotations, or missing, add it...
			modified := broker.DeepCopy()
			modifiedAnnotations := modified.ObjectMeta.GetAnnotations()
			if modifiedAnnotations == nil {
				modifiedAnnotations = make(map[string]string, 1)
			}
			if _, present := modifiedAnnotations[eventing.BrokerClassKey]; !present {
				modifiedAnnotations[eventing.BrokerClassKey] = eventing.ChannelBrokerClassValue
				modified.ObjectMeta.SetAnnotations(modifiedAnnotations)
			}
			patch, err := duck.CreateMergePatch(broker, modified)
			if err != nil {
				logger.Warnf("Failed to create patch for \"%s/%s\" : %v", broker.Namespace, broker.Name, err)
				return err
			}
			logger.Infof("Patch: %q", string(patch))
			// If there is nothing to patch, we are good, just return.
			// Empty patch is {}, hence we check for that.
			if len(patch) <= 2 {
				logger.Warnf("no diffs found...")
				continue
			}
			patched, err := brokerClient.Patch(broker.Name, types.MergePatchType, patch)
			if err != nil {
				logger.Warnf("Failed to patch \"%s/%s\" : %v", broker.Namespace, broker.Name, err)
			}
			logger.Infof("Patched \"%s/%s\" successfully new Annotations: %+v", broker.Namespace, broker.Name, patched.ObjectMeta.GetAnnotations())
		}
	}
	return nil
}
