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

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/adapter/apiserver"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
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
	TracingConfig string
}

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// ApiServer Sources.
func MakeReceiveAdapter(args *ReceiveAdapterArgs) (*v1.Deployment, error) {
	replicas := int32(1)

	env, err := makeEnv(args)
	if err != nil {
		return nil, err
	}

	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Source.Namespace,
			Name:      kmeta.ChildName(fmt.Sprintf("apiserversource-%s-", args.Source.Name), string(args.Source.GetUID())),
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
							Env:   env,
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}},
						},
					},
				},
			},
		},
	}, nil
}

func makeEnv(args *ReceiveAdapterArgs) ([]corev1.EnvVar, error) {
	cfg := &apiserver.Config{
		Namespace:     args.Source.Namespace,
		Resources:     make([]apiserver.ResourceWatch, 0, len(args.Source.Spec.Resources)),
		ResourceOwner: args.Source.Spec.ResourceOwner,
		EventMode:     args.Source.Spec.EventMode,
	}

	for _, r := range args.Source.Spec.Resources {
		gv, err := schema.ParseGroupVersion(r.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse APIVersion, %s", err)
		}
		gvr, _ := meta.UnsafeGuessKindToResource(gv.WithKind(r.Kind))

		rw := apiserver.ResourceWatch{GVR: gvr}

		if r.LabelSelector != nil {
			selector, _ := metav1.LabelSelectorAsSelector(r.LabelSelector)
			rw.LabelSelector = selector.String()
		}

		cfg.Resources = append(cfg.Resources, rw)
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
		Name:  "SYSTEM_NAMESPACE",
		Value: system.Namespace(),
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
	}, {
		Name:  "K_TRACING_CONFIG",
		Value: args.TracingConfig,
	},
	}, nil
}
