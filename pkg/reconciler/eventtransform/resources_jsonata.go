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
	JsonataResourcesLabelKey   = "eventing.knative.dev/event-transform-jsonata"
	JsonataResourcesLabelValue = "true"
	JsonataExpressionHashKey   = "eventing.knative.dev/event-transform-jsonata-expression-hash"
	JsonataResourcesNameSuffix = "jsonata"
	JsonataExpressionDataKey   = "jsonata-expression"
	JsonataExpressionPath      = "/etc/jsonata"

	JsonataResourcesSelector = JsonataResourcesLabelKey + "=" + JsonataResourcesLabelValue
)

func jsonataExpressionConfigMap(_ context.Context, transform *eventing.EventTransform) corev1.ConfigMap {
	expression := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix),
			Namespace:   transform.GetNamespace(),
			Labels:      jsonataLabels(transform),
			Annotations: transform.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(transform),
			},
		},
		Data: map[string]string{
			JsonataExpressionDataKey: transform.Spec.Transformations.Jsonata.Expression,
		},
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
			Annotations: transform.Annotations,
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

	// When the expression changes, this rolls out a new deployment with the latest ConfigMap data.
	hash := sha256.Sum256([]byte(transform.Spec.Transformations.Jsonata.Expression))
	d.Annotations[JsonataExpressionHashKey] = base64.StdEncoding.EncodeToString(hash[:])

	return d
}

func jsonataSinkBindingName(transform *eventing.EventTransform) string {
	return kmeta.ChildName(transform.Name, JsonataResourcesNameSuffix)
}

func jsonataSinkBinding(_ context.Context, transform *eventing.EventTransform) sourcesv1.SinkBinding {
	name := jsonataSinkBindingName(transform)
	sb := sourcesv1.SinkBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   transform.GetNamespace(),
			Labels:      jsonataLabels(transform),
			Annotations: transform.Annotations,
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
	labels := make(map[string]string, len(transform.Labels)+2)
	for k, v := range transform.Labels {
		labels[k] = v
	}
	labels[JsonataResourcesLabelKey] = JsonataResourcesLabelValue
	labels[NameLabelKey] = transform.GetName()
	return labels
}
