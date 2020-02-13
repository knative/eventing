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

package resources

import (
	configsv1alpha1 "knative.dev/eventing/pkg/apis/configs/v1alpha1"
	"knative.dev/pkg/kmeta"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMapArgs struct {
	Original             *corev1.ConfigMap
	ConfigMapPropagation *configsv1alpha1.ConfigMapPropagation
}

func MakeConfigMap(args ConfigMapArgs) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MakeCopyConfigMapName(args.ConfigMapPropagation.Name, args.Original.Name),
			Namespace: args.ConfigMapPropagation.Namespace,
			Labels: map[string]string{
				PropagationLabelKey: PropagationLabelValueCopy,
				CopyLabelKey:        MakeCopyConfigMapLabel(args.Original.Namespace, args.Original.Name),
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.ConfigMapPropagation),
			},
		},
		Data: args.Original.Data,
	}
}

func MakeCopyConfigMapName(configMapPropagationName, configMapName string) string {
	return configMapPropagationName + "-" + configMapName
}

// MakeCopyCOnfigMapLabel uses '-' to separate namespace and configmap name instead of '/',
// for label values only accept '-', '.', '_'.
func MakeCopyConfigMapLabel(originalNamespace, originalName string) string {
	return originalNamespace + "-" + originalName
}
