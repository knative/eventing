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
	"fmt"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/eventing/pkg/reconciler/integration"

	"knative.dev/pkg/kmeta"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	commonintegrationv1alpha1 "knative.dev/eventing/pkg/apis/common/integration/v1alpha1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/apis/sinks/v1alpha1"
)

const (
	testName      = "test-integrationsink"
	testNamespace = "test-namespace"
	testUID       = "test-uid"
)

func TestNewSQSContainerSink(t *testing.T) {
	const timerImage = "quay.io/sqs-image"
	t.Setenv("INTEGRATION_SINK_AWS_SQS_IMAGE", timerImage)
	sink := &v1alpha1.IntegrationSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			UID:       testUID,
		},
		Spec: v1alpha1.IntegrationSinkSpec{
			Aws: &v1alpha1.Aws{
				SQS: &commonintegrationv1alpha1.AWSSQS{
					AWSCommon: commonintegrationv1alpha1.AWSCommon{
						Region: "us-east-1",
					},
					Arn:                "arn:aws:sqs:us-east-1:123456789012:knative-integration-source",
					DeleteAfterRead:    true,
					AutoCreateQueue:    false,
					Host:               "amazonaws.com",
					Protocol:           "https",
					Greedy:             false,
					Delay:              500,
					MaxMessagesPerPoll: 1,
					WaitTimeSeconds:    0,
					VisibilityTimeout:  0,
				},
				Auth: &commonintegrationv1alpha1.Auth{
					ServiceAccountName: "aws-service-account",
				},
			},
		},
	}

	want := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-deployment", testName),
			Namespace: testNamespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(sink),
			},
			Labels: integration.Labels(testName),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: integration.Labels(testName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: integration.Labels(testName),
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: fmt.Sprintf("%s-server-tls", testName),
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: fmt.Sprintf("%s-server-tls", testName),
									Optional:   ptr.To(true),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "sink",
							Image:           timerImage,
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
							Env: []corev1.EnvVar{
								{Name: "QUARKUS_HTTP_SSL_CERTIFICATE_KEY-FILES", Value: "/etc/test-integrationsink-server-tls/tls.key"},
								{Name: "QUARKUS_HTTP_SSL_CERTIFICATE_FILES", Value: "/etc/test-integrationsink-server-tls/tls.crt"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_REGION", Value: "us-east-1"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_OVERRIDEENDPOINT", Value: "false"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_QUEUE_NAME_OR_ARN", Value: "arn:aws:sqs:us-east-1:123456789012:knative-integration-source"},

								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_DELETEAFTERREAD", Value: "true"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_AUTOCREATEQUEUE", Value: "false"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_GREEDY", Value: "false"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_DELAY", Value: "500"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_MAXMESSAGESPERPOLL", Value: "1"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_AMAZONAWSHOST", Value: "amazonaws.com"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_PROTOCOL", Value: "https"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_WAITTIMESECONDS", Value: "0"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_VISIBILITYTIMEOUT", Value: "0"},
								{Name: "CAMEL_KAMELET_AWS_SQS_SINK_USE_DEFAULT_CREDENTIALS_PROVIDER", Value: "true"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      fmt.Sprintf("%s-server-tls", testName),
									MountPath: "/etc/" + fmt.Sprintf("%s-server-tls", testName),
									ReadOnly:  true,
								},
							},
							LivenessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/q/health/live",
										Port:   intstr.FromInt32(8080),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      10,
							},
							ReadinessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/q/health/ready",
										Port:   intstr.FromInt32(8080),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      10,
							},
							StartupProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/q/health/started",
										Port:   intstr.FromInt32(8080),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      10,
							},
						},
					},
					ServiceAccountName: "aws-service-account",
				},
			},
		},
	}

	got, _ := MakeDeploymentSpec(sink, "unused", feature.Flags{}, nil)
	sortOpts := []cmp.Option{
		cmp.Transformer("SortEnvVars", func(in corev1.Container) corev1.Container {
			out := in
			if len(out.Env) > 0 {
				sort.Slice(out.Env, func(i, j int) bool {
					return out.Env[i].Name < out.Env[j].Name
				})
			}
			return out
		}),
	}
	if diff := cmp.Diff(want, got, sortOpts...); diff != "" {
		t.Errorf("MakeDeploymentSpec() mismatch (-want +got):\n%s", diff)
	}
}
