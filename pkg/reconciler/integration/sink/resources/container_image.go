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
	"encoding/json"
	"fmt"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/ptr"
	commonv1a1 "knative.dev/eventing/pkg/apis/common/integration/v1alpha1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/certificates"
	alpha1 "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	v1alpha1listers "knative.dev/eventing/pkg/client/listers/sinks/v1alpha1"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/reconciler/integration"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"
)

const (
	AuthProxyRolebindingName = "eventing-auth-proxy"
)

func MakeDeploymentSpec(sink *v1alpha1.IntegrationSink, authProxyImage string, featureFlags feature.Flags, trustBundleConfigmapLister corev1listers.ConfigMapLister, eventPolicyLister alpha1.EventPolicyLister) (*appsv1.Deployment, error) {
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(sink.Name),
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
							Name: certificates.CertificateName(sink.Name),
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: certificates.CertificateName(sink.Name),
									Optional:   ptr.To(true),
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
									Name:      certificates.CertificateName(sink.Name),
									MountPath: "/etc/" + certificates.CertificateName(sink.Name),
									ReadOnly:  true,
								},
							},
						},
					},
					ServiceAccountName: makeServiceAccountName(sink),
				},
			},
		},
	}

	if featureFlags.IsOIDCAuthentication() {
		// add auth-proxy

		proxyVars, err := makeAuthProxyEnv(sink, featureFlags, eventPolicyLister)
		if err != nil {
			return nil, fmt.Errorf("failed to make auth proxy env vars: %w", err)
		}

		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers, corev1.Container{
			Name:            "auth-proxy",
			Image:           authProxyImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: 3128,
					Protocol:      corev1.ProtocolTCP,
					Name:          "http",
				},
				{
					ContainerPort: 3129,
					Protocol:      corev1.ProtocolTCP,
					Name:          "https",
				},
			},
			Env: proxyVars,
			VolumeMounts: []corev1.VolumeMount{
				{
					// server certs, as the auth-proxy uses the same certs as the underlying sink
					Name:      certificates.CertificateName(sink.Name),
					MountPath: "/etc/" + certificates.CertificateName(sink.Name),
					ReadOnly:  true,
				},
			},
		})

		// add trustbundles directly, so auth-proxies tokenverifier does not need the trustbundleconfigmap lister for oidc discovery
		podspec, err := eventingtls.AddTrustBundleVolumes(trustBundleConfigmapLister, deploy, &deploy.Spec.Template.Spec)
		if err != nil {
			return nil, fmt.Errorf("failed to add trust bundle volumes: %w", err)
		}
		deploy.Spec.Template.Spec = *podspec

		// don't expose ports on sink container, as traffic should reach only auth-proxy
		deploy.Spec.Template.Spec.Containers[0].Ports = nil
	}

	return deploy, nil
}

func MakeService(sink *v1alpha1.IntegrationSink) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(sink.Name),
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
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "http",
					},
				},
				{
					Name:     "https",
					Protocol: corev1.ProtocolTCP,
					Port:     443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "https",
					},
				},
			},
		},
	}
}

func MakeAuthProxyRoleBindings(sink *v1alpha1.IntegrationSink, sinkLister v1alpha1listers.IntegrationSinkLister, features feature.Flags) (*rbacv1.RoleBinding, error) {
	rb := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AuthProxyRolebindingName,
			Namespace: system.Namespace(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "knative-eventing-auth-proxy",
		},
	}

	// now we need to get the SA names for all the deployed IntegrationSink pods
	sinks, err := sinkLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("error listing sinks: %w", err)
	}
	sinks = append(sinks, sink) //add the current sink too, as this could not exist yet in the cluster

	serviceAccounts := map[types.NamespacedName]struct{}{}
	for _, s := range sinks {
		serviceAccounts[types.NamespacedName{
			Namespace: s.Namespace,
			Name:      "default", //TODO: get the real SA of the pod, as it could be that the integrationsink pod does not run under the default SA
		}] = struct{}{}
	}

	for sa := range serviceAccounts {
		rb.Subjects = append(rb.Subjects, rbacv1.Subject{
			Kind:      "ServiceAccount",
			Namespace: sa.Namespace,
			Name:      sa.Name,
		})
	}

	return &rb, nil
}

func makeAuthProxyEnv(sink *v1alpha1.IntegrationSink, featureFlags feature.Flags, eventPolicyLister alpha1.EventPolicyLister) ([]corev1.EnvVar, error) {
	sinkAddress := eventingtls.GetHttpsAddress(sink.Status.Addresses)
	if sinkAddress == nil {
		sinkAddress = sink.Status.Address
	}

	policies, err := auth.SubjectWithFiltersFromPolicyRef(eventPolicyLister, sink.Namespace, sink.Status.Policies)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth proxy policies env vars: %w", err)
	}

	policiesJson, err := json.Marshal(policies)
	if err != nil {
		return nil, fmt.Errorf("failed to parse policies for auth proxy env vars: %w", err)
	}

	envVars := []corev1.EnvVar{
		{
			Name:  "TARGET_HTTP_PORT",
			Value: "8080",
		},
		{
			Name:  "TARGET_HTTPS_PORT",
			Value: "8443",
		},
		{
			Name:  "PROXY_HTTP_PORT",
			Value: "3128",
		},
		{
			Name:  "PROXY_HTTPS_PORT",
			Value: "3129",
		},
		{
			Name:  "SYSTEM_NAMESPACE",
			Value: system.Namespace(),
		},
		{
			Name:  "AUTH_POLICIES",
			Value: string(policiesJson),
		},
		{
			Name:  "SINK_NAMESPACE",
			Value: sink.Namespace,
		},
	}

	if sinkAddress != nil && sinkAddress.URL != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SINK_URI",
			Value: sinkAddress.URL.String(),
		})
	}

	if !featureFlags.IsDisabledTransportEncryption() {
		envVars = append(envVars, []corev1.EnvVar{
			{
				Name:  "SINK_TLS_CERT_FILE",
				Value: "/etc/" + certificates.CertificateName(sink.Name) + "/tls.crt",
			},
			{
				Name:  "SINK_TLS_KEY_FILE",
				Value: "/etc/" + certificates.CertificateName(sink.Name) + "/tls.key",
			}, {
				Name:  "SINK_TLS_CA_FILE",
				Value: "/etc/" + certificates.CertificateName(sink.Name) + "/ca.crt",
			},
		}...)
	}

	if sinkAddress != nil && sinkAddress.Audience != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SINK_AUDIENCE",
			Value: *sinkAddress.Audience,
		})
	}

	return envVars, nil
}

func makeEnv(sink *v1alpha1.IntegrationSink, featureFlags feature.Flags) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	// Transport encryption environment variables
	if !featureFlags.IsDisabledTransportEncryption() {
		envVars = append(envVars, []corev1.EnvVar{
			{
				Name:  "QUARKUS_HTTP_SSL_CERTIFICATE_FILES",
				Value: "/etc/" + certificates.CertificateName(sink.Name) + "/tls.crt",
			},
			{
				Name:  "QUARKUS_HTTP_SSL_CERTIFICATE_KEY-FILES",
				Value: "/etc/" + certificates.CertificateName(sink.Name) + "/tls.key",
			},
		}...)
	}

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
		} else {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "CAMEL_KAMELET_AWS_S3_SINK_USE_DEFAULT_CREDENTIALS_PROVIDER",
				Value: "true",
			})
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
		} else {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "CAMEL_KAMELET_AWS_SQS_SINK_USE_DEFAULT_CREDENTIALS_PROVIDER",
				Value: "true",
			})
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
		} else {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "CAMEL_KAMELET_AWS_SNS_SINK_USE_DEFAULT_CREDENTIALS_PROVIDER",
				Value: "true",
			})
		}
		return envVars
	}

	// If no valid configuration is found, return empty envVars
	return envVars
}

func makeServiceAccountName(sink *v1alpha1.IntegrationSink) string {
	if sink.Spec.Aws != nil && sink.Spec.Aws.Auth != nil && sink.Spec.Aws.Auth.ServiceAccountName != "" {
		return sink.Spec.Aws.Auth.ServiceAccountName
	}
	return ""
}

func selectImage(sink *v1alpha1.IntegrationSink) string {
	// Injected in ./config/core/deployments/controller.yaml
	switch {
	case sink.Spec.Log != nil:
		return os.Getenv("INTEGRATION_SINK_LOG_IMAGE")
	case sink.Spec.Aws != nil && sink.Spec.Aws.S3 != nil:
		return os.Getenv("INTEGRATION_SINK_AWS_S3_IMAGE")
	case sink.Spec.Aws != nil && sink.Spec.Aws.SQS != nil:
		return os.Getenv("INTEGRATION_SINK_AWS_SQS_IMAGE")
	case sink.Spec.Aws != nil && sink.Spec.Aws.SNS != nil:
		return os.Getenv("INTEGRATION_SINK_AWS_SNS_IMAGE")
	default:
		return ""
	}
}
