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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeChannel(source *v1alpha1.Source, org *v1alpha1.Channel, args *HeartBeatArguments) (*v1alpha1.Channel, error) {
	channel := &v1alpha1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Channel",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.Name,
			Namespace: args.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*controller.NewControllerRef(source, false),
			},
		},
		Spec: v1alpha1.ChannelSpec{
			Provisioner: &v1alpha1.ProvisionerReference{
				Ref: &corev1.ObjectReference{
					Name:       "in-memory-bus-provisioner",
					APIVersion: "eventing.knative.dev/v1alpha1",
					Kind:       "ClusterProvisioner",
				},
			},
		},
	}
	if org != nil {
		channel.Spec.Generation = org.Spec.Generation
	}
	return channel, nil
}
