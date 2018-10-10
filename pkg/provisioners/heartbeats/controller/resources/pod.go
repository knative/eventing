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
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"

	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	image = "gcr.io/plori-nicholss/cmd-f2e9e0e7e0a0f8a66b5b6f8d85f4c98b@sha256:d64b5fd7b4c8c5fdc70cca7732fcf384d3069a4a1762a69e58d701a040345aed"
)

func MakePod(source *v1alpha1.Source, org *corev1.Pod, channel *v1alpha1.Channel, args *HeartBeatArguments) (*corev1.Pod, error) {

	if channel == nil || channel.Status.Sinkable.DomainInternal == "" {
		return nil, fmt.Errorf("channel not ready")
	}

	remote := fmt.Sprintf("--remote=http://%s", channel.Status.Sinkable.DomainInternal)
	period := ""
	if args.Period > 0 {
		period = fmt.Sprintf("--period=%d", args.Period)
	}
	label := ""
	if args.Label != "" {
		period = fmt.Sprintf("--label=%s", args.Label)
	}

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.Name,
			Namespace:    args.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*controller.NewControllerRef(source, false),
			},
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "true",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				{
					Name:  "heartbeat",
					Image: image,
					Args: []string{
						remote, period, label,
					},
				},
			},
		},
	}
	return pod, nil
}
