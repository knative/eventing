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

import (
	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type ContainerArguments struct {
	Source      *v1alpha1.ContainerSource
	Name        string
	Namespace   string
	Template    *corev1.PodTemplateSpec
	Sink        string
	Annotations map[string]string
	Labels      map[string]string

	// TODO(jingweno): The following fields are to be deprecated
	// Use `Template` instead
	Image              string
	Args               []string
	Env                []corev1.EnvVar
	ServiceAccountName string
}
