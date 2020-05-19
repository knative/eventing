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
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	//	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
)

// Upgrade upgrades all the brokers by seeing if they have Spec.ChannelTemplate
// and if they do, create a ConfigMap in that namespace with the contents of the
// Spec.ChannelTemplate, that's owned by the Broker, then update the Broker
// to have Spec.Config to point to the newly created ConfigMap and remove
// the ChannelTemplate.
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
			logger.Infof("Broker \"%s/%s\" does not require updating", broker.Namespace, broker.Name)
			continue
		}

		// Ok, there are differences, apply the patch
		logger.Infof("Patching Broker \"%s/%s\" with %q", broker.Namespace, broker.Name, string(patch))
		patched, err := brokerClient.Patch(broker.Name, types.MergePatchType, patch)
		if err != nil {
			logger.Warnf("Failed to patch \"%s/%s\" : %v", broker.Namespace, broker.Name, err)
			return err
		}
		logger.Infof("Patched \"%s/%s\" successfully new Spec: %+v", broker.Namespace, broker.Name, patched.Spec)
	}
	return nil
}

// Process a broker, create a ConfigMap representing the ChannelTemplate
// and point the Config to it.
// Returns non-empty patch bytes if a patch is necessary.
func processBroker(ctx context.Context, broker v1alpha1.Broker) ([]byte, error) {
	logger := logging.FromContext(ctx)
	if broker.Spec.ChannelTemplate == nil || broker.Spec.ChannelTemplate.Kind == "" {
		logger.Infof("Broker \"%s/%s\" is not using channeltemplate, skipping...", broker.Namespace, broker.Name)
		return []byte{}, nil
	}

	modified := broker.DeepCopy()

	// If the Broker already has a Config, don't modify it, just nil out the channel template.
	if broker.Spec.Config == nil || broker.Spec.Config.Name == "" {
		cm, err := createConfigMap(ctx, broker)
		if err != nil {
			return []byte{}, err
		}
		modified.Spec.Config = cm
	}
	modified.Spec.ChannelTemplate = nil

	patch, err := duck.CreateMergePatch(broker, modified)
	if err != nil {
		logger.Warnf("Failed to create patch for \"%s/%s\" : %v", broker.Namespace, broker.Name, err)
		return []byte{}, err
	}
	logger.Infof("Patch for \"%s/%s\": %q", broker.Namespace, broker.Name, string(patch))
	// If there is nothing to patch, we are good, just return.
	// Empty patch is {}, hence we check for that.
	if len(patch) <= 2 {
		return []byte{}, nil
	}
	return patch, nil
}

func createConfigMap(ctx context.Context, broker v1alpha1.Broker) (*duckv1.KReference, error) {
	// Generating the spec portion is a bit goofy cause we have turn the runtime raw into
	// a yaml blob that we stick into the configmap (with proper indentation).
	data := ""
	if broker.Spec.ChannelTemplate.Spec != nil {
		bytes, err := yaml.JSONToYAML(broker.Spec.ChannelTemplate.Spec.Raw)
		if err != nil {
			return nil, err
		}
		logging.FromContext(ctx).Infof("BYTES: %q", string(bytes))
		data = fmt.Sprintf(`
apiVersion: %q
kind: %q
spec:
  %s
`, broker.Spec.ChannelTemplate.APIVersion, broker.Spec.ChannelTemplate.Kind, strings.ReplaceAll(strings.TrimSpace(string(bytes)), "\n", "\n  "))
	} else {
		data = fmt.Sprintf(`
apiVersion: %q
kind: %q
`, broker.Spec.ChannelTemplate.APIVersion, broker.Spec.ChannelTemplate.Kind)
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "broker-upgrade-auto-gen-config-" + broker.Name,
			Namespace: broker.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(&broker),
			},
		},
		Data: map[string]string{"channelTemplateSpec": data},
	}

	logging.FromContext(ctx).Infof("Creating configmap: %+v", cm)
	_, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(broker.Namespace).Create(cm)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to create broker config map \"%s/%s\": %v", broker.Namespace, broker.Name, err)
		return nil, err
	}
	return &duckv1.KReference{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Namespace:  broker.Namespace,
		Name:       "broker-upgrade-auto-gen-config-" + broker.Name,
	}, nil
}
