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

package prober

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/eventing/test/lib/resources"
)

func kService(meta metav1.ObjectMeta, spec corev1.PodSpec) *unstructured.Unstructured {
	if len(spec.Containers) != 1 {
		panic("kService func supports PodSpec with 1 container only")
	}
	container := spec.Containers[0]
	if len(spec.Volumes) != 1 || len(container.VolumeMounts) != 1 {
		panic("kService func supports PodSpec with 1 volume only")
	}
	volume := spec.Volumes[0]
	volmount := container.VolumeMounts[0]
	obj := map[string]interface{}{
		"apiVersion": resources.KServiceType.APIVersion,
		"kind":       resources.KServiceType.Kind,
		"metadata": map[string]interface{}{
			"name":      meta.Name,
			"namespace": meta.Namespace,
			"labels":    meta.Labels,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": meta.Annotations,
				},
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{{
						"name":  container.Name,
						"image": container.Image,
						"volumeMounts": []map[string]interface{}{{
							"name":      volmount.Name,
							"mountPath": volmount.MountPath,
							"readOnly":  true,
						}},
						"readinessProbe": map[string]interface{}{
							"httpGet": map[string]interface{}{
								"path": container.ReadinessProbe.HTTPGet.Path,
							},
						},
					}},
					"volumes": []map[string]interface{}{{
						"name": volume.Name,
						"configMap": map[string]interface{}{
							"name": volume.ConfigMap.Name,
						},
					}},
				},
			},
		},
	}
	return &unstructured.Unstructured{Object: obj}
}
