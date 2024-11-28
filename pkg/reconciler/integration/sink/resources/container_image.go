/*
Copyright 2024 The Knative Authors

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	commonv1a1 "knative.dev/eventing/pkg/apis/common/integration/v1alpha1"
	"knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/reconciler/integration"
	"knative.dev/pkg/kmeta"
)

func MakeDeploymentSpec(sink *v1alpha1.IntegrationSink) *appsv1.Deployment {

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(sink),
			Namespace: sink.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(sink),
			},
			Labels: integration.Labels(sink.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: integration.Labels(sink.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: integration.Labels(sink.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "sink",
							Image:           selectImage(sink),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{{
								ContainerPort: 8080,
								Protocol:      corev1.ProtocolTCP,
								Name:          "http",
							}},
							Env: makeEnv(sink),
						},
					},
				},
			},
		},
	}

	return deploy
}

func MakeService(sink *v1alpha1.IntegrationSink) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(sink),
			Namespace: sink.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(sink),
			},
			Labels: integration.Labels(sink.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: integration.Labels(sink.Name),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
			},
		},
	}
}

func DeploymentName(sink *v1alpha1.IntegrationSink) string {
	return kmeta.ChildName(sink.Name, "-deployment")
}

func makeEnv(sink *v1alpha1.IntegrationSink) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	// Log environment variables
	if sink.Spec.Log != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct("CAMEL_KAMELET_LOG_SINK", *sink.Spec.Log)...)
		return envVars
	}

	// Handle secret name only if AWS is configured
	var secretName string
	if sink.Spec.Aws != nil && sink.Spec.Aws.Auth != nil && sink.Spec.Aws.Auth.Secret != nil && sink.Spec.Aws.Auth.Secret.Ref != nil {
		secretName = sink.Spec.Aws.Auth.Secret.Ref.Name
	}

	// AWS S3 environment variables
	if sink.Spec.Aws != nil && sink.Spec.Aws.S3 != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct("CAMEL_KAMELET_AWS_S3_SINK", *sink.Spec.Aws.S3)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SINK_ACCESSKEY", commonv1a1.AwsAccessKey, secretName),
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_S3_SINK_SECRETKEY", commonv1a1.AwsSecretKey, secretName),
			}...)
		}
		return envVars
	}

	// AWS SQS environment variables
	if sink.Spec.Aws != nil && sink.Spec.Aws.SQS != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct("CAMEL_KAMELET_AWS_SQS_SINK", *sink.Spec.Aws.SQS)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SINK_ACCESSKEY", commonv1a1.AwsAccessKey, secretName),
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_SQS_SINK_SECRETKEY", commonv1a1.AwsSecretKey, secretName),
			}...)
		}
		return envVars
	}

	// If no valid configuration is found, return empty envVars
	return envVars
}

func selectImage(sink *v1alpha1.IntegrationSink) string {
	if sink.Spec.Log != nil {
		return "gcr.io/knative-nightly/log-sink:latest"
	}
	if sink.Spec.Aws != nil {
		if sink.Spec.Aws.S3 != nil {
			return "gcr.io/knative-nightly/aws-s3-source:latest"
		}
		if sink.Spec.Aws.SQS != nil {
			return "gcr.io/knative-nightly/aws-sqs-source:latest"
		}
	}
	return ""
}
