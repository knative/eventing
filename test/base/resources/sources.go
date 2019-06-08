/*
Copyright 2019 The Knative Authors

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

// This file contains functions that construct Sources resources.

import (
	sourcesv1alpha1 "github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	pkgTest "github.com/knative/pkg/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CronJobSourceOption enables further configuration of a CronJobSource.
type CronJobSourceOption func(*sourcesv1alpha1.CronJobSource)

// ContainerSourceOption enables further configuration of a ContainerSource.
type ContainerSourceOption func(*sourcesv1alpha1.ContainerSource)

// WithSinkServiceForCronJobSource returns an option that adds a Kubernetes Service sink for the given CronJobSource.
func WithSinkServiceForCronJobSource(name string) CronJobSourceOption {
	return func(cjs *sourcesv1alpha1.CronJobSource) {
		cjs.Spec.Sink = pkgTest.CoreV1ObjectReference(ServiceKind, CoreAPIVersion, name)
	}
}

// WithServiceAccountForCronJobSource returns an option that adds a ServiceAccount for the given CronJobSource.
func WithServiceAccountForCronJobSource(saName string) CronJobSourceOption {
	return func(cjs *sourcesv1alpha1.CronJobSource) {
		cjs.Spec.ServiceAccountName = saName
	}
}

// CronJobSource returns a CronJob EventSource.
func CronJobSource(
	name,
	schedule,
	data string,
	options ...CronJobSourceOption,
) *sourcesv1alpha1.CronJobSource {
	cronJobSource := &sourcesv1alpha1.CronJobSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: sourcesv1alpha1.CronJobSourceSpec{
			Schedule: schedule,
			Data:     data,
		},
	}
	for _, option := range options {
		option(cronJobSource)
	}
	return cronJobSource
}

// WithTemplateForContainerSource returns an option that adds a template for the given ContainerSource.
func WithTemplateForContainerSource(template *corev1.PodTemplateSpec) ContainerSourceOption {
	return func(cs *sourcesv1alpha1.ContainerSource) {
		cs.Spec.Template = template
	}
}

// WithSinkServiceForContainerSource returns an option that adds a Kubernetes Service sink for the given ContainerSource.
func WithSinkServiceForContainerSource(name string) ContainerSourceOption {
	return func(cs *sourcesv1alpha1.ContainerSource) {
		cs.Spec.Sink = pkgTest.CoreV1ObjectReference(ServiceKind, CoreAPIVersion, name)
	}
}

// ContainerSource returns a Container EventSource.
func ContainerSource(
	name string,
	options ...ContainerSourceOption,
) *sourcesv1alpha1.ContainerSource {
	containerSource := &sourcesv1alpha1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, option := range options {
		option(containerSource)
	}
	return containerSource
}

// ContainerSourceBasicTemplate returns a basic template that can be used in ContainerSource.
func ContainerSourceBasicTemplate(
	name,
	namespace,
	imageName string,
	args []string,
) *corev1.PodTemplateSpec {
	envVars := []corev1.EnvVar{
		corev1.EnvVar{
			Name:  "POD_NAME",
			Value: name,
		},
		corev1.EnvVar{
			Name:  "POD_NAMESPACE",
			Value: namespace,
		},
	}

	podTemplateSpec := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				corev1.Container{
					Name:            imageName,
					Image:           pkgTest.ImagePath(imageName),
					ImagePullPolicy: corev1.PullAlways,
					Args:            args,
					Env:             envVars,
				},
			},
		},
	}
	return podTemplateSpec
}
