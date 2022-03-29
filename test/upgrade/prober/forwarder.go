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
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/upgrade/prober/wathola/forwarder"
)

func (p *prober) deployForwarder() {
	p.log.Infof("Deploy forwarder knative service: %v", forwarder.Name)
	serving := p.client.Dynamic.Resource(resources.KServicesGVR).Namespace(p.client.Namespace)
	service := p.forwarderKService(forwarder.Name, p.client.Namespace)
	if _, err := serving.Create(p.config.Ctx, service, metav1.CreateOptions{}); err != nil {
		p.client.T.Fatal(err)
	}

	sc := p.servingClient()
	testlib.WaitFor(fmt.Sprintf("forwarder ksvc be ready: %v", forwarder.Name), func() error {
		return duck.WaitForKServiceReady(sc, forwarder.Name, p.client.Namespace)
	})

	if p.config.Serving.ScaleToZero {
		testlib.WaitFor(fmt.Sprintf("forwarder scales to zero: %v", forwarder.Name), func() error {
			return duck.WaitForKServiceScales(p.config.Ctx, sc, forwarder.Name, p.client.Namespace, func(scale int) bool {
				return scale == 0
			})
		})
	}
}

func (p *prober) removeForwarder() {
	p.log.Infof("Remove forwarder knative service: %v", forwarder.Name)
	serving := p.client.Dynamic.Resource(resources.KServicesGVR).Namespace(p.client.Namespace)
	err := serving.Delete(p.config.Ctx, forwarder.Name, metav1.DeleteOptions{})
	p.ensureNoError(err)
}

func (p *prober) forwarderKService(name, namespace string) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": resources.KServiceType.APIVersion,
		"kind":       resources.KServiceType.Kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]string{
				"networking.knative.dev/visibility": "cluster-local",
			},
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{{
						"name":  forwarder.Name,
						"image": p.config.ImageResolver(forwarder.Name),
						"volumeMounts": []map[string]interface{}{{
							"name":      p.config.ConfigMapName,
							"mountPath": p.config.ConfigMountPoint,
							"readOnly":  true,
						}},
						"readinessProbe": map[string]interface{}{
							"httpGet": map[string]interface{}{
								"path": p.config.HealthEndpoint,
							},
						},
					}},
					"volumes": []map[string]interface{}{{
						"name": p.config.ConfigMapName,
						"configMap": map[string]interface{}{
							"name": p.config.ConfigMapName,
						},
					}},
				},
			},
		},
	}
	return &unstructured.Unstructured{Object: obj}
}
