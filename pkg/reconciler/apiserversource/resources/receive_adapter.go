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
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/adapter/apiserver"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/kmeta"
)

// ReceiveAdapterArgs are the arguments needed to create a ApiServer Receive Adapter.
// Every field is required.
type ReceiveAdapterArgs struct {
	Image         string
	Source        *v1alpha2.ApiServerSource
	Labels        map[string]string
	SinkURI       string
	MetricsConfig string
	LoggingConfig string
}

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// ApiServer Sources.
func MakeReceiveAdapter(args *ReceiveAdapterArgs) *v1.Deployment {
	replicas := int32(1)
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Source.Namespace,
			Name:      utils.GenerateFixedName(args.Source, fmt.Sprintf("apiserversource-%s", args.Source.Name)),
			Labels:    args.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: args.Labels,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false", // needs to talk to the api server.
					},
					Labels: args.Labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.Source.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: args.Image,
							Env:   makeEnv(args),
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}},
						},
					},
				},
			},
		},
	}
}

func makeEnv(args *ReceiveAdapterArgs) []corev1.EnvVar {
	cfg := &apiserver.Config{
		Namespace:     args.Source.Namespace,
		Resources:     make([]schema.GroupVersionResource, len(args.Source.Spec.Resources)),
		ResourceOwner: args.Source.Spec.ResourceOwner,
		EventMode:     args.Source.Spec.EventMode,
	}

	if args.Source.Spec.LabelSelector != nil {
		cfg.LabelSelector = args.Source.Spec.LabelSelector.String()
	}

	for _, r := range args.Source.Spec.Resources {
		if r.APIVersion == nil {
			panic("nil api version") // TODO: not really do this.
		}
		gv, err := schema.ParseGroupVersion(*r.APIVersion)
		if err != nil {
			panic(err) // TODO: not really do this.
		}

		if r.Kind == nil {
			panic("nil kind") // TODO: not really do this.
		}
		gvr, _ := meta.UnsafeGuessKindToResource(gv.WithKind(*r.Kind))

		cfg.Resources = append(cfg.Resources, gvr)
	}

	config := "{}"
	if b, err := json.Marshal(cfg); err == nil {
		config = string(b)
	}

	return []corev1.EnvVar{{
		Name:  "K_SINK",
		Value: args.SinkURI,
	}, {
		Name:  "K_SOURCE_CONFIG",
		Value: config,
	}, {
		Name: "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}, {
		Name:  "NAME",
		Value: args.Source.Name,
	}, {
		Name:  "METRICS_DOMAIN",
		Value: "knative.dev/eventing",
	}, {
		Name:  "K_METRICS_CONFIG",
		Value: args.MetricsConfig,
	}, {
		Name:  "K_LOGGING_CONFIG",
		Value: args.LoggingConfig,
	}}
}
