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
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/kmeta"
)

const (
	sourceLabelKey = "sources.eventing.knative.dev/containerSource"
)

func MakeDeployment(args ContainerArguments) *appsv1.Deployment {
	template := args.Template
	if template == nil {
		template = &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				ServiceAccountName: args.ServiceAccountName,
				Containers: []corev1.Container{
					{
						Name:            "source",
						Image:           args.Image,
						Args:            args.Args,
						Env:             args.Env,
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
			},
		}
	}

	if template.ObjectMeta.Labels == nil {
		template.ObjectMeta.Labels = make(map[string]string)
	}
	template.ObjectMeta.Labels[sourceLabelKey] = args.Name

	containers := []corev1.Container{}
	for _, c := range template.Spec.Containers {
		sink, hasSinkArg := sinkArg(c.Args)
		if sink == "" {
			sink = args.Sink
		}

		// if sink is already in the provided, don't attempt to add
		if !hasSinkArg {
			c.Args = append(c.Args, "--sink="+sink)
		}

		c.Env = append(c.Env, corev1.EnvVar{Name: "SINK", Value: sink})
		containers = append(containers, c)
	}
	template.Spec.Containers = containers

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.GenerateFixedName(args.Source, fmt.Sprintf("containersource-%s", args.Source.Name)),
			Namespace: args.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
			Labels: map[string]string{
				sourceLabelKey: args.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					sourceLabelKey: args.Name,
				},
			},
			Template: *template,
		},
	}

	// Then wire through any annotations from the source.
	if args.Annotations != nil {
		if deploy.Spec.Template.ObjectMeta.Annotations == nil {
			deploy.Spec.Template.ObjectMeta.Annotations = make(map[string]string, len(args.Annotations))
		}
		for k, v := range args.Annotations {
			deploy.Spec.Template.ObjectMeta.Annotations[k] = v
		}
	}

	// Then wire through any labels from the source. Do not allow to override
	// our source name. This seems like it would be way errorprone by allowing
	// the matchlabels then to not match, or we'd have to force them to match, etc.
	// just don't allow it.
	if args.Labels != nil {
		for k, v := range args.Labels {
			if k != sourceLabelKey {
				deploy.Spec.Template.ObjectMeta.Labels[k] = v
			}
		}
	}
	return deploy
}

func sinkArg(args []string) (string, bool) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--sink=") {
			return strings.Replace(arg, "--sink=", "", -1), true
		}
	}

	return "", false
}
