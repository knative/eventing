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
	"context"
	"fmt"

	"github.com/wavesoftware/go-ensure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	pkgTest "knative.dev/pkg/test"
)

var forwarderName = "wathola-forwarder"

func (p *prober) deployForwarder(ctx context.Context) {
	p.log.Info("Deploy forwarder knative service: ", forwarderName)
	serving := p.client.Dynamic.Resource(resources.KServicesGVR).Namespace(p.client.Namespace)
	service := p.forwarderKService(forwarderName, p.client.Namespace)
	_, err := serving.Create(context.Background(), service, metav1.CreateOptions{})
	ensure.NoError(err)

	sc := p.servingClient()
	testlib.WaitFor(fmt.Sprint("forwarder knative service be ready: ", forwarderName), func() error {
		return duck.WaitForKServiceReady(sc, forwarderName, p.client.Namespace)
	})

	if p.config.Serving.ScaleToZero {
		testlib.WaitFor(fmt.Sprint("forwarder scales to zero: ", forwarderName), func() error {
			return duck.WaitForKServiceScales(ctx, sc, forwarderName, p.client.Namespace, func(scale int) bool {
				return scale == 0
			})
		})
	}
}

func (p *prober) removeForwarder() {
	p.log.Info("Remove forwarder knative service: ", forwarderName)
	serving := p.client.Dynamic.Resource(resources.KServicesGVR).Namespace(p.client.Namespace)
	err := serving.Delete(context.Background(), forwarderName, metav1.DeleteOptions{})
	ensure.NoError(err)
}

func (p *prober) forwarderKService(name, namespace string) *unstructured.Unstructured {
	return kService(metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels: map[string]string{
			"serving.knative.dev/visibility": "cluster-local",
		},
	}, corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  "forwarder",
			Image: pkgTest.ImagePath(forwarderName),
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      p.config.ConfigMapName,
					MountPath: p.config.ConfigMountPoint,
					ReadOnly:  true,
				},
			},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: p.config.HealthEndpoint,
					},
				},
			},
		}},
		Volumes: []corev1.Volume{{
			Name: p.config.ConfigMapName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: p.config.ConfigMapName,
					},
				},
			},
		}},
	})
}
