/*
Copyright 2018 The Knative Authors

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
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeDeployment(source *v1alpha1.Source, org *appsv1.Deployment, channel *v1alpha1.Channel, args *ContainerArguments) (*appsv1.Deployment, error) {

	if channel == nil || channel.Status.Sinkable.DomainInternal == "" {
		return nil, fmt.Errorf("channel not ready")
	}

	containerArgs := []string(nil)
	if args != nil {
		containerArgs = make([]string, 0, len(args.Args)+1)
		for k, v := range args.Args {
			containerArgs = append(containerArgs, fmt.Sprintf("--%s=%q", k, v))
		}
	}
	remote := fmt.Sprintf("--remote=http://%s", channel.Status.Sinkable.DomainInternal)
	containerArgs = append(containerArgs, remote)

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.Name + "-",
			Namespace:    args.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*controller.NewControllerRef(source, false),
			},
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func() *int32 { var i int32 = 1; return &i }(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"source": args.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"source": args.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:            "source",
							Image:           args.Image,
							Args:            containerArgs,
							ImagePullPolicy: corev1.PullAlways,
						},
					},
				},
			},
		},
	}
	return deploy, nil
}
