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

package testing

import (
	corev1 "k8s.io/api/core/v1"
)

type PodSpecOption func(*corev1.PodSpec)

func MakePodSpec(name, image string, opts ...PodSpecOption) corev1.PodSpec {
	p := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  name,
				Image: image,
			},
		},
	}

	for _, opt := range opts {
		opt(p)
	}
	return *p
}

func WithPodContainerPort(name string, port int32) PodSpecOption {
	return func(p *corev1.PodSpec) {
		p.Containers[0].Ports = []corev1.ContainerPort{
			{
				Name:          name,
				ContainerPort: port,
			},
		}
	}
}

func WithPodContainerProbes(liveness, readiness *corev1.Probe) PodSpecOption {
	return func(p *corev1.PodSpec) {
		p.Containers[0].LivenessProbe = liveness
		p.Containers[0].ReadinessProbe = readiness
	}
}
