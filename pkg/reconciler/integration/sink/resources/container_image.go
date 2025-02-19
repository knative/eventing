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
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/reconciler/integration"
	"knative.dev/pkg/kmeta"
)

var sinkImageMap = map[string]string{
	"log":     "gcr.io/knative-nightly/log-sink:latest",
	"aws-s3":  "gcr.io/knative-nightly/aws-s3-sink:latest",
	"aws-sqs": "gcr.io/knative-nightly/aws-sqs-sink:latest",
	"aws-sns": "gcr.io/knative-nightly/aws-sns-sink:latest",
}

func MakeDeploymentSpec(sink *v1alpha1.IntegrationSink, featureFlags feature.Flags) *appsv1.Deployment {
	t := true

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
					Volumes: []corev1.Volume{
						{
							Name: CertificateName(sink),
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: CertificateName(sink),
									Optional:   &t,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "sink",
							Image:           selectImage(sink),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
									Name:          "http",
								},
								{
									ContainerPort: 8443,
									Protocol:      corev1.ProtocolTCP,
									Name:          "https",
								}},
							Env: makeEnv(sink, featureFlags),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      CertificateName(sink),
									MountPath: "/etc/" + CertificateName(sink),
									ReadOnly:  true,
								},
							},
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
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       443,
					TargetPort: intstr.IntOrString{IntVal: 8443},
				},
			},
		},
	}
}

func makeEnv(sink *v1alpha1.IntegrationSink, featureFlags feature.Flags) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	//// Transport encryption environment variables
	//if !featureFlags.IsDisabledTransportEncryption() {
	//	envVars = append(envVars, []corev1.EnvVar{
	//		{
	//			Name:  "QUARKUS_HTTP_SSL_CERTIFICATE_FILES",
	//			Value: "/etc/" + CertificateName(sink) + "/tls.crt",
	//		},
	//		{
	//			Name:  "QUARKUS_HTTP_SSL_CERTIFICATE_KEY-FILES",
	//			Value: "/etc/" + CertificateName(sink) + "/tls.key",
	//		},
	//	}...)
	//}

	// No HTTP with strict TLS
	if featureFlags.IsStrictTransportEncryption() {
		envVars = append(envVars, []corev1.EnvVar{
			{
				Name:  "QUARKUS_HTTP_INSECURE_REQUESTS",
				Value: "disabled",
			},
		}...)
	}

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

	// AWS SNS environment variables
	if sink.Spec.Aws != nil && sink.Spec.Aws.SNS != nil {
		envVars = append(envVars, integration.GenerateEnvVarsFromStruct("CAMEL_KAMELET_AWS_SNS_SINK", *sink.Spec.Aws.SNS)...)
		if secretName != "" {
			envVars = append(envVars, []corev1.EnvVar{
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_SNS_SINK_ACCESSKEY", commonv1a1.AwsAccessKey, secretName),
				integration.MakeSecretEnvVar("CAMEL_KAMELET_AWS_SNS_SINK_SECRETKEY", commonv1a1.AwsSecretKey, secretName),
			}...)
		}
		return envVars
	}

	// If no valid configuration is found, return empty envVars
	return envVars
}

func selectImage(sink *v1alpha1.IntegrationSink) string {
	switch {
	case sink.Spec.Log != nil:
		return sinkImageMap["log"]
	case sink.Spec.Aws != nil && sink.Spec.Aws.S3 != nil:
		return sinkImageMap["aws-s3"]
	case sink.Spec.Aws != nil && sink.Spec.Aws.SQS != nil:
		return sinkImageMap["aws-sqs"]
	case sink.Spec.Aws != nil && sink.Spec.Aws.SNS != nil:
		return sinkImageMap["aws-sns"]
	default:
		return ""
	}
}
