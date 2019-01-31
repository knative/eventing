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
	"encoding/json"
	"fmt"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ActivatorArgs struct {
	Broker             *eventingv1alpha1.Broker
	Image              string
	ServiceAccountName string
}

func MakeActivator(args *ActivatorArgs) (*servingv1alpha1.Service, error) {
	templateJson, err := json.Marshal(args.Broker.Spec.ChannelTemplate)
	if err != nil {
		return nil, err
	}

	return &servingv1alpha1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      fmt.Sprintf("%s-broker-activator", args.Broker.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(args.Broker, schema.GroupVersionKind{
					Group:   args.Broker.GroupVersionKind().Group,
					Version: args.Broker.GroupVersionKind().Version,
					Kind:    args.Broker.GroupVersionKind().Kind,
				}),
			},
		},
		Spec: servingv1alpha1.ServiceSpec{
			RunLatest: &servingv1alpha1.RunLatestType{
				Configuration: servingv1alpha1.ConfigurationSpec{
					RevisionTemplate: servingv1alpha1.RevisionTemplateSpec{
						Spec: servingv1alpha1.RevisionSpec{
							ServiceAccountName: args.ServiceAccountName,
							Container: v1.Container{
								Image: args.Image,
								Env: []v1.EnvVar{
									{
										Name:  "CHANNEL_TEMPLATE",
										Value: string(templateJson),
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}
