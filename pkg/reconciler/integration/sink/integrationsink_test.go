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
	"fmt"
	"sync/atomic"

	cmlisters "github.com/cert-manager/cert-manager/pkg/client/listers/certmanager/v1"

	"knative.dev/eventing/pkg/certificates"

	"knative.dev/eventing/pkg/reconciler/integration"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	sinksv1alpha1 "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/sinks/v1alpha1/integrationsink"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	"context"

	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	. "knative.dev/eventing/pkg/reconciler/testing/v1alpha1"

	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	"testing"
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
		}
	}

	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: sink.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(sink),
			},
			Labels: integration.Labels(sink.Name),
		},
		Status: status,
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
						},
					},
				},
			},
		},
	}
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

func makeDeploymentStatus(ready *corev1.ConditionStatus) *appsv1.DeploymentStatus {
	return &appsv1.DeploymentStatus{
		Conditions: []appsv1.DeploymentCondition{{
			Type:   appsv1.DeploymentAvailable,
			Status: *ready,
		}},
		Replicas: 1,
	}
}
