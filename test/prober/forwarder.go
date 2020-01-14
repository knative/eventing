/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package prober

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/test/common"
)

var (
	servicesCR = schema.GroupVersionResource{
		Group:    "serving.knative.dev",
		Version:  "v1alpha1",
		Resource: "services",
	}
	servingType = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: servicesCR.GroupVersion().String(),
	}
	forwarderName = "wathola-forwarder"
)

func (p *prober) deployForwarder() {
	p.log.Infof("Deploy forwarder knative service: %v", forwarderName)
	serving := p.client.Dynamic.Resource(servicesCR).Namespace(p.client.Namespace)
	service := forwarderKService(forwarderName, p.client.Namespace)
	_, err := serving.Create(service, metav1.CreateOptions{})
	common.NoError(err)

	waitFor(fmt.Sprintf("forwarder ksvc be ready: %v", forwarderName), func() error {
		return p.waitForKServiceReady(forwarderName, p.client.Namespace)
	})

	if p.config.Serving.ScaleToZero {
		waitFor(fmt.Sprintf("forwarder scales to zero: %v", forwarderName), func() error {
			return p.waitForKServiceScale(forwarderName, p.client.Namespace, func(scale *int32) bool {
				return *scale == 0
			})
		})
	}
}

func (p *prober) removeForwarder() {
	p.log.Infof("Remove forwarder knative service: %v", forwarderName)
	serving := p.client.Dynamic.Resource(servicesCR).Namespace(p.client.Namespace)
	err := serving.Delete(forwarderName, &metav1.DeleteOptions{})
	common.NoError(err)
}

func forwarderKService(name, namespace string) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": servingType.APIVersion,
		"kind":       servingType.Kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]string{
				"serving.knative.dev/visibility": "cluster-local",
			},
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{{
						"name":  "forwarder",
						"image": fmt.Sprintf("quay.io/cardil/wathola-forwarder:%v", Version),
						"volumeMounts": []map[string]interface{}{{
							"name":      configName,
							"mountPath": "/.config/wathola",
							"readOnly":  true,
						}},
						"readinessProbe": map[string]interface{}{
							"httpGet": map[string]interface{}{
								"path": "/healthz",
							},
						},
					}},
					"volumes": []map[string]interface{}{{
						"name": configName,
						"secret": map[string]interface{}{
							"secretName": configName,
						},
					}},
				},
			},
		},
	}
	return &unstructured.Unstructured{Object: obj}
}
