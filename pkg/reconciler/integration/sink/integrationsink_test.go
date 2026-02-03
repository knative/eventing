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

package sink

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	cmlisters "github.com/cert-manager/cert-manager/pkg/client/listers/certmanager/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/apis/feature"
	sinksv1alpha1 "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/certificates"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sinks/v1alpha1/integrationsink"
	"knative.dev/eventing/pkg/reconciler/integration"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	. "knative.dev/eventing/pkg/reconciler/testing/v1alpha1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
)

const (
	// testNamespace is the namespace used for testing.
	sinkName = "test-integration-sink"
	sinkUID  = "1234-5678-90"
	testNS   = "test-namespace"

	logSinkImage = "quay.io/fake-image/log-sink"
)

var (
	conditionTrue  = corev1.ConditionTrue
	deploymentName = fmt.Sprintf("%s-deployment", sinkName)

	sinkAddressable = duckv1.Addressable{
		Name: ptr.To("http"),
		URL: &apis.URL{
			Scheme: "http",
			Host:   network.GetServiceHostname(deploymentName, testNS),
		},
	}
)

func TestReconcile(t *testing.T) {
	t.Setenv("INTEGRATION_SINK_LOG_IMAGE", logSinkImage)

	table := TableTest{
		{
			Name: "bad work queue key",
			Key:  "too/many/parts",
		},
		{
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "error creating deployment",
			Objects: []runtime.Object{
				NewIntegrationSink(sinkName, testNS,
					WithIntegrationSinkUID(sinkUID),
					WithIntegrationSinkSpec(makeIntegrationSinkSpec()),
				),
			},
			Key: testNS + "/" + sinkName,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "creating new Deployment: inducing failure for %s %s", "create", "deployments"),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewIntegrationSink(sinkName, testNS,
					WithIntegrationSinkUID(sinkUID),
					WithIntegrationSinkSpec(makeIntegrationSinkSpec()),
					WithInitIntegrationSinkConditions,
					WithIntegrationSinkTrustBundlePropagatedReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeDeployment(NewIntegrationSink(sinkName, testNS,
					WithIntegrationSinkUID(sinkUID),
					WithIntegrationSinkSpec(makeIntegrationSinkSpec())),
					nil),
			},
		}, {
			Name: "successfully reconciled and ready",
			Objects: []runtime.Object{
				NewIntegrationSink(sinkName, testNS,
					WithIntegrationSinkUID(sinkUID),
					WithIntegrationSinkSpec(makeIntegrationSinkSpec()),
				),
				makeDeployment(NewIntegrationSink(sinkName, testNS,
					WithIntegrationSinkUID(sinkUID),
					WithIntegrationSinkSpec(makeIntegrationSinkSpec())),
					&conditionTrue),
				makeService(deploymentName, testNS),
			},
			Key: testNS + "/" + sinkName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, sinkReconciled, `IntegrationSink reconciled: "%s/%s"`, testNS, sinkName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewIntegrationSink(sinkName, testNS,
					WithIntegrationSinkUID(sinkUID),
					WithIntegrationSinkAddressableReady(),
					WithIntegrationSinkAddress(sinkAddressable),
					WithIntegrationSinkSpec(makeIntegrationSinkSpec()),
					WithIntegrationSinkEventPoliciesReadyBecauseOIDCDisabled(),
					WithIntegrationSinkTrustBundlePropagatedReady(),
					WithInitIntegrationSinkConditions,
					WithIntegrationSinkPropagateDeploymenteStatus(makeDeploymentStatus(&conditionTrue)),
				),
			}},
		}, {
			Name: "creates deployment with auth-proxy and RoleBindings when OIDC is enabled",
			Objects: []runtime.Object{
				NewIntegrationSink(sinkName, testNS,
					WithIntegrationSinkUID(sinkUID),
					WithIntegrationSinkSpec(makeIntegrationSinkSpec()),
				),
			},
			Key: testNS + "/" + sinkName,
			Ctx: feature.ToContext(context.Background(), feature.Flags{
				feature.OIDCAuthentication: feature.Enabled,
			}),
			WantCreates: []runtime.Object{
				makeEventPolicyRoleBinding(),
				makeConfigMapAccessRoleBinding(),
				makeDeploymentWithAuthProxy(NewIntegrationSink(sinkName, testNS,
					WithIntegrationSinkUID(sinkUID),
					WithIntegrationSinkSpec(makeIntegrationSinkSpec())),
					nil),
				makeService(deploymentName, testNS),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, deploymentCreated, "Deployment created %q", deploymentName),
				Eventf(corev1.EventTypeNormal, serviceCreated, "Service created %q", deploymentName),
				Eventf(corev1.EventTypeNormal, sinkReconciled, `IntegrationSink reconciled: "%s/%s"`, testNS, sinkName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewIntegrationSink(sinkName, testNS,
					WithIntegrationSinkUID(sinkUID),
					WithIntegrationSinkAddressableReady(),
					WithIntegrationSinkAddress(duckv1.Addressable{
						Name: ptr.To("http"),
						URL: &apis.URL{
							Scheme: "http",
							Host:   network.GetServiceHostname(deploymentName, testNS),
						},
						Audience: ptr.To("sinks.knative.dev/integrationsink/" + testNS + "/" + sinkName),
					}),
					WithIntegrationSinkSpec(makeIntegrationSinkSpec()),
					WithIntegrationSinkEventPoliciesReady("DefaultAuthorizationMode", `Default authz mode is ""`),
					WithIntegrationSinkTrustBundlePropagatedReady(),
					WithInitIntegrationSinkConditions,
					WithIntegrationSinkPropagateDeploymenteStatus(&appsv1.Deployment{
						Status: appsv1.DeploymentStatus{},
					}),
				),
			}},
			SkipNamespaceValidation: true,
		}}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)

		cmCertificatesListerAtomic := &atomic.Pointer[cmlisters.CertificateLister]{}
		cmCertificatesLister := listers.GetCertificateLister()
		cmCertificatesListerAtomic.Store(&cmCertificatesLister)

		r := &Reconciler{
			kubeClientSet:              fakekubeclient.Get(ctx),
			deploymentLister:           listers.GetDeploymentLister(),
			serviceLister:              listers.GetServiceLister(),
			secretLister:               listers.GetSecretLister(),
			cmCertificateLister:        cmCertificatesListerAtomic,
			eventPolicyLister:          listers.GetEventPolicyLister(),
			trustBundleConfigMapLister: listers.GetConfigMapLister(),
			rolebindingLister:          listers.GetRoleBindingLister(),
			integrationSinkLister:      listers.GetIntegrationSinkLister(),
		}

		return integrationsink.NewReconciler(ctx, logging.FromContext(ctx), fakeeventingclient.Get(ctx), listers.GetIntegrationSinkLister(), controller.GetEventRecorder(ctx), r)
	},
		true,
		logger,
	))
}

func makeDeployment(sink *sinksv1alpha1.IntegrationSink, ready *corev1.ConditionStatus) runtime.Object {
	t := true

	status := appsv1.DeploymentStatus{}
	if ready != nil {
		status.Conditions = []appsv1.DeploymentCondition{
			{
				Type:   appsv1.DeploymentAvailable,
				Status: *ready,
			},
		}
		if *ready == corev1.ConditionTrue {
			status.ReadyReplicas = 1
			status.UpdatedReplicas = 1
			status.AvailableReplicas = 1
			status.Replicas = 1
			status.ObservedGeneration = 1
		}
	}

	objectMeta := metav1.ObjectMeta{
		Name:      deploymentName,
		Namespace: sink.Namespace,
		OwnerReferences: []metav1.OwnerReference{
			*kmeta.NewControllerRef(sink),
		},
		Labels: integration.Labels(sink.Name),
	}
	// Simulate what Kubernetes API server would set for existing deployments
	if ready != nil {
		objectMeta.Generation = 1
	}

	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: objectMeta,
		Status:     status,
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
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
									Optional:   &t,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "sink",
							Image:           logSinkImage,
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
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "QUARKUS_HTTP_SSL_CERTIFICATE_FILES",
									Value: "/etc/test-integration-sink-server-tls/tls.crt",
								},
								{
									Name:  "QUARKUS_HTTP_SSL_CERTIFICATE_KEY-FILES",
									Value: "/etc/test-integration-sink-server-tls/tls.key",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_LEVEL",
									Value: "info",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_LOGMASK",
									Value: "false",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_MULTILINE",
									Value: "false",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_SHOWALLPROPERTIES",
									Value: "false",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_SHOWBODY",
									Value: "true",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_SHOWBODYTYPE",
									Value: "true",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_SHOWEXCHANGEPATTERN",
									Value: "false",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_SHOWHEADERS",
									Value: "true",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_SHOWPROPERTIES",
									Value: "false",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_SHOWSTREAMS",
									Value: "false",
								},
								{
									Name:  "CAMEL_KAMELET_LOG_SINK_SHOWCACHEDSTREAMS",
									Value: "false",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      certificates.CertificateName(sink.Name),
									MountPath: "/etc/" + certificates.CertificateName(sink.Name),
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
				},
			},
		},
	}
	return d
}

func makeDeploymentWithAuthProxy(sink *sinksv1alpha1.IntegrationSink, ready *corev1.ConditionStatus) runtime.Object {
	d := makeDeployment(sink, ready).(*appsv1.Deployment)

	// Remove ports from sink container (auth-proxy becomes the entry point)
	d.Spec.Template.Spec.Containers[0].Ports = nil

	// Add auth-proxy container
	authProxyContainer := corev1.Container{
		Name:            "auth-proxy",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports: []corev1.ContainerPort{
			{ContainerPort: 3128, Protocol: corev1.ProtocolTCP, Name: "http"},
			{ContainerPort: 3129, Protocol: corev1.ProtocolTCP, Name: "https"},
			{ContainerPort: 8090, Protocol: corev1.ProtocolTCP, Name: "health"},
		},
		Env: []corev1.EnvVar{
			{Name: "TARGET_HTTP_PORT", Value: "8080"},
			{Name: "TARGET_HTTPS_PORT", Value: "8443"},
			{Name: "PROXY_HTTP_PORT", Value: "3128"},
			{Name: "PROXY_HTTPS_PORT", Value: "3129"},
			{Name: "PROXY_HEALTH_PORT", Value: "8090"},
			{Name: "SYSTEM_NAMESPACE", Value: system.Namespace()},
			{Name: "PARENT_API_VERSION", Value: "sinks.knative.dev/v1alpha1"},
			{Name: "PARENT_KIND", Value: "IntegrationSink"},
			{Name: "PARENT_NAME", Value: sink.Name},
			{Name: "PARENT_NAMESPACE", Value: sink.Namespace},
			{Name: "SINK_NAMESPACE", Value: sink.Namespace},
			{Name: "SINK_URI", Value: "http://" + deploymentName + "." + sink.Namespace + ".svc.cluster.local"},
			{Name: "SINK_TLS_CERT_FILE", Value: "/etc/" + certificates.CertificateName(sink.Name) + "/tls.crt"},
			{Name: "SINK_TLS_KEY_FILE", Value: "/etc/" + certificates.CertificateName(sink.Name) + "/tls.key"},
			{Name: "SINK_TLS_CA_FILE", Value: "/etc/" + certificates.CertificateName(sink.Name) + "/ca.crt"},
			{Name: "SINK_AUDIENCE", Value: "sinks.knative.dev/integrationsink/" + sink.Namespace + "/" + sink.Name},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      certificates.CertificateName(sink.Name),
				MountPath: "/etc/" + certificates.CertificateName(sink.Name),
				ReadOnly:  true,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/readyz",
					Port:   intstr.FromInt32(8090),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
			FailureThreshold:    3,
			TimeoutSeconds:      5,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt32(8090),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
			TimeoutSeconds:      5,
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/readyz",
					Port:   intstr.FromInt32(8090),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 0,
			PeriodSeconds:       2,
			SuccessThreshold:    1,
			FailureThreshold:    30,
			TimeoutSeconds:      5,
		},
	}

	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, authProxyContainer)
	return d
}

func makeService(name, namespace string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    integration.Labels(sinkName),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "sinks.knative.dev/v1alpha1",
					Kind:               "IntegrationSink",
					Name:               sinkName,
					UID:                sinkUID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Spec: corev1.ServiceSpec{
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
			Selector: integration.Labels(sinkName),
		},
	}
}

func makeIntegrationSinkSpec() sinksv1alpha1.IntegrationSinkSpec {
	return sinksv1alpha1.IntegrationSinkSpec{
		Log: &sinksv1alpha1.Log{
			Level:        "info",
			ShowHeaders:  true,
			ShowBody:     true,
			ShowBodyType: true,
		},
	}
}

func makeDeploymentStatus(ready *corev1.ConditionStatus) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: *ready,
			}},
			Replicas:           1,
			UpdatedReplicas:    1,
			AvailableReplicas:  1,
			ObservedGeneration: 1,
		},
	}
}

func makeEventPolicyRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sinkName + "-eventpolicy-reader",
			Namespace: testNS,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "sinks.knative.dev/v1alpha1",
					Kind:               "IntegrationSink",
					Name:               sinkName,
					UID:                sinkUID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
			Labels: integration.Labels(sinkName),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "knative-eventing-eventpolicy-reader",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: testNS,
				Name:      "default",
			},
		},
	}
}

func makeConfigMapAccessRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eventing-auth-proxy",
			Namespace: system.Namespace(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "knative-eventing-auth-proxy",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: testNS,
				Name:      "default",
			},
		},
	}
}
