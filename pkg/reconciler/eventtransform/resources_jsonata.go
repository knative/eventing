/*
Copyright 2025 The Knative Authors

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

package eventtransform

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/tracker"

	eventing "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
)

const (
	JsonataResourcesLabelKey      = "eventing.knative.dev/event-transform-jsonata"
	JsonataResourcesLabelValue    = "true"
	JsonataExpressionHashKey      = "eventing.knative.dev/event-transform-jsonata-expression-hash"
	JsonataResourcesNameSuffix    = "-jsonata"
	JsonataExpressionDataKey      = "jsonata-expression"
	JsonataReplyExpressionDataKey = "jsonata-expression-reply"

	JsonataExpressionPath = "/etc/jsonata"

	JsonataResourcesSelector = JsonataResourcesLabelKey + "=" + JsonataResourcesLabelValue
)

func jsonataExpressionConfigMap(_ context.Context, transform *eventing.EventTransform) corev1.ConfigMap {
	expression := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix),
			Namespace: transform.GetNamespace(),
			Labels:    jsonataLabels(transform),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		Data: map[string]string{
			JsonataExpressionDataKey: transform.Spec.EventTransformations.Jsonata.Expression,
		},
	}

	if transform.Spec.Reply != nil && transform.Spec.Reply.Jsonata != nil {
		expression.Data[JsonataReplyExpressionDataKey] = transform.Spec.Reply.Jsonata.Expression
	}

	return expression
}

func jsonataDeployment(_ context.Context, expression *corev1.ConfigMap, transform *eventing.EventTransform) appsv1.Deployment {
	image := os.Getenv("EVENT_TRANSFORM_JSONATA_IMAGE")
	if image == "" {
		panic("EVENT_TRANSFORM_JSONATA_IMAGE must be set")
	}

	d := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix),
			Namespace:   transform.GetNamespace(),
			Labels:      jsonataLabels(transform),
			Annotations: make(map[string]string, 2),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					JsonataResourcesLabelKey: JsonataResourcesLabelValue,
					NameLabelKey:             transform.GetName(),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						JsonataResourcesLabelKey: JsonataResourcesLabelValue,
						NameLabelKey:             transform.GetName(),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "jsonata-event-transform",
							Image: image,
							Env: []corev1.EnvVar{
								{
									Name:  "JSONATA_TRANSFORM_FILE_NAME",
									Value: filepath.Join(JsonataExpressionPath, JsonataExpressionDataKey),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      expression.GetName(),
									ReadOnly:  true,
									MountPath: JsonataExpressionPath,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: expression.GetName(),
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: expression.GetName(),
									},
									Optional: ptr.Bool(false),
								},
							},
						},
					},
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 0,
					},
				},
			},
		},
	}

	// hashPayload is used to detect and roll out a new deployment when expressions change.
	hashPayload := transform.Spec.EventTransformations.Jsonata.Expression

	if transform.Spec.Reply != nil {
		if transform.Spec.Reply.Discard != nil {
			if *transform.Spec.Reply.Discard {
				d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "JSONATA_DISCARD_RESPONSE_BODY",
					Value: "true",
				})
			} else {
				d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "JSONATA_DISCARD_RESPONSE_BODY",
					Value: "false",
				})
			}
		} else {
			d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  "JSONATA_DISCARD_RESPONSE_BODY",
				Value: "false",
			})
		}

		if transform.Spec.Reply.Jsonata != nil {
			d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  "JSONATA_RESPONSE_TRANSFORM_FILE_NAME",
				Value: filepath.Join(JsonataExpressionPath, JsonataReplyExpressionDataKey),
			})
			hashPayload += transform.Spec.Reply.Jsonata.Expression
		}
	}

	hash := sha256.Sum256([]byte(hashPayload))
	d.Annotations[JsonataExpressionHashKey] = base64.StdEncoding.EncodeToString(hash[:])

	return d
}

func jsonataService(_ context.Context, transform *eventing.EventTransform) corev1.Service {
	s := corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:        kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix),
			Namespace:   transform.GetNamespace(),
			Labels:      jsonataLabels(transform),
			Annotations: make(map[string]string, 2),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:        "http",
					Protocol:    corev1.ProtocolTCP,
					AppProtocol: ptr.String("http"),
					Port:        80,
					TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			Selector: map[string]string{
				JsonataResourcesLabelKey: JsonataResourcesLabelValue,
				NameLabelKey:             transform.GetName(),
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	return s
}

func jsonataSinkBindingName(transform *eventing.EventTransform) string {
	return kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix)
}

func jsonataSinkBinding(_ context.Context, transform *eventing.EventTransform) sourcesv1.SinkBinding {
	name := jsonataSinkBindingName(transform)
	sb := sourcesv1.SinkBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: transform.GetNamespace(),
			Labels:    jsonataLabels(transform),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		Spec: sourcesv1.SinkBindingSpec{
			SourceSpec: duckv1.SourceSpec{
				Sink: *transform.Spec.Sink.DeepCopy(),
			},
			BindingSpec: duckv1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Namespace:  transform.GetNamespace(),
					Name:       name,
				},
			},
		},
	}

	return sb
}

func jsonataLabels(transform *eventing.EventTransform) map[string]string {
	labels := make(map[string]string, 2)
	labels[JsonataResourcesLabelKey] = JsonataResourcesLabelValue
	labels[NameLabelKey] = transform.GetName()
	return labels
}
