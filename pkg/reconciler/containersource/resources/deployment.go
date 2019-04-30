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

	"github.com/knative/pkg/kmeta"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const sourceLabelKey = "eventing.knative.dev/source"

func MakeDeployment(args ContainerArguments) *appsv1.Deployment {

	containerArgs := args.Args

	// if sink is already in the provided args.Args, don't attempt to add
	if !args.SinkInArgs {
		remote := fmt.Sprintf("--sink=%s", args.Sink)
		containerArgs = append(containerArgs, remote)
	}

	env := append(args.Env, corev1.EnvVar{Name: "SINK", Value: sinkArg(args)})

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.Name + "-",
			Namespace:    args.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					sourceLabelKey: args.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: map[string]string{
						sourceLabelKey: args.Name,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:            "source",
							Image:           args.Image,
							Args:            containerArgs,
							Env:             env,
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
				},
			},
		},
	}

	// Then wire through any annotations from the source. Not a bug by allowing
	// the container to override Istio injection.
	if args.Annotations != nil {
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

func sinkArg(args ContainerArguments) string {
	if args.SinkInArgs {
		for _, a := range args.Args {
			if strings.HasPrefix(a, "--sink=") {
				return strings.Replace(a, "--sink=", "", -1)
			}
		}
	}
	return args.Sink
}
