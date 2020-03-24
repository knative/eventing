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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
)

type ContainerSourceArgs struct {
	Source      *v1alpha1.ContainerSource
	SinkURI     *apis.URL
	CeOverrides string
	Labels      map[string]string
}

func MakeDeployment(args *ContainerSourceArgs) *appsv1.Deployment {
	template := args.Source.Spec.Template

	for i := range template.Spec.Containers {
		template.Spec.Containers[i].Env = append(template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "K_SINK",
			Value: args.SinkURI.String(),
		})
		template.Spec.Containers[i].Env = append(template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "K_CE_OVERRIDES",
			Value: args.CeOverrides,
		})
	}

	if template.Labels == nil {
		template.Labels = make(map[string]string)
	}
	for k, v := range args.Labels {
		template.Labels[k] = v
	}

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kmeta.ChildName(fmt.Sprintf("containersource-%s-", args.Source.Name), string(args.Source.UID)),
			Namespace: args.Source.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
			Labels: args.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: args.Labels,
			},
			Template: template,
		},
	}
	return deploy
}
