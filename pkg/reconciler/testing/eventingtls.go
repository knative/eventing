/*
Copyright 2023 The Knative Authors

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

package testing

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"knative.dev/eventing/pkg/apis/feature"
)

func MakeTLSPermissiveFeatureConfigMap(namespace string) runtime.Object {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-features",
			Namespace: namespace,
		},
		Data: map[string]string{
			feature.TransportEncryption: string(feature.Permissive),
		},
	}
}

func MakeTLSStrictFeatureConfigMap(namespace string) runtime.Object {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-features",
			Namespace: namespace,
		},
		Data: map[string]string{
			feature.TransportEncryption: string(feature.Strict),
		},
	}
}
