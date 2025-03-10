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
	"fmt"
	"sync/atomic"
	"testing"

	cmapis "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/feature"
	sources "knative.dev/eventing/pkg/apis/sources/v1"
	cmclient "knative.dev/eventing/pkg/client/certmanager/injection/client/fake"
	cmlisters "knative.dev/eventing/pkg/client/certmanager/listers/certmanager/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/eventtransform"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	. "knative.dev/eventing/pkg/reconciler/testing/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS   = "test-namespace"
	testName = "test-event-transform"
)

var (
	testKey = fmt.Sprintf("%s/%s", testNS, testName)

	sink = duckv1.Destination{
		URI: apis.HTTP("example.com"),
	}
	sink2 = duckv1.Destination{
		URI: apis.HTTP("example2.com"),
	}
)

func TestReconcile(t *testing.T) {
	t.Setenv("EVENT_TRANSFORM_JSONATA_IMAGE", "quay.io/event-transform")

	ctx := context.Background()
	logger := logtesting.TestLogger(t)
	ctx = logging.WithLogger(ctx, logger)

	cw := reconcilersource.WatchConfigurations(
		ctx,
		"eventtransform",
		configmap.NewStaticWatcher(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "config-logging"},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "config-observability"},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "config-tracing"},
			},
		),
	)

	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
			Objects: []runtime.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
		},
		{
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
			Objects: []runtime.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
		},
		{
			Name: "Object not found",
			Key:  testKey,
			Objects: []runtime.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
		},
		{
			Name: "Reconcile initial loop",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{}),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx),
				jsonataTestDeployment(ctx, cw),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
				eventJsonataDeploymentCreated(),
			},
			WantErr: true, // skip key
		},
		{
			Name: "Reconcile second loop, deployment ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithJsonataWaitingForServiceEndpoints(),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
			},
			WantErr: true, // skip key, waiting for endpoints
		},
		{
			Name: "Reconcile second loop, endpoint present but not ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
						},
					},
				},
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithJsonataWaitingForServiceEndpoints(),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
			},
			WantErr: true, // skip key, waiting for endpoints
		},
		{
			Name: "Reconcile second loop, endpoint ready, addressable",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
			},
		},
		{
			Name: "Reconcile update deployment",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Spec.Template.Spec.Containers = nil
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  0,
						Replicas:            0,
						UpdatedReplicas:     0,
						ReadyReplicas:       0,
						AvailableReplicas:   0,
						UnavailableReplicas: 0,
					}),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: jsonataTestDeployment(ctx, cw)},
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
				eventJsonataDeploymentUpdated(),
			},
			WantErr: true, // skip key "error"
		},
		{
			Name: "Reconcile expression config map update",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataExpressionTestConfigMap(ctx, func(configMap *corev1.ConfigMap) {
					configMap.Data[JsonataExpressionDataKey] = `{"name": name}`
				}),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataTestService(ctx),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: jsonataExpressionTestConfigMap(ctx)},
			},
			WantEvents: []string{
				eventJsonataConfigMapUpdated(),
				eventJsonataServiceCreated(),
			},
		},
		{
			Name: "Reconcile service update",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx, func(service *corev1.Service) {
					service.Spec.Type = corev1.ServiceTypeLoadBalancer
				}),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: jsonataTestService(ctx)},
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceUpdated(),
			},
		},
		{
			Name: "Reconcile no op",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx),
				jsonataExpressionTestConfigMap(ctx),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
		},
		{
			Name: "Reconcile sink binding",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithEventTransformSink(sink),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx),
				jsonataExpressionTestConfigMap(ctx),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink),
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataSinkBindingStatus(sources.SinkBindingStatus{}),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						Replicas:           1,
						AvailableReplicas:  1,
						ObservedGeneration: 1,
						UpdatedReplicas:    1,
						ReadyReplicas:      1,
					}),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataTestSinkBinding(ctx),
			},
			WantEvents: []string{
				eventJsonataSinkBindingCreated(),
			},
			WantErr: true,
		},
		{
			Name: "Reconcile sink binding updated",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithEventTransformSink(sink2),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx),
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestSinkBinding(ctx),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink2),
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataSinkBindingStatus(sources.SinkBindingStatus{}),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						Replicas:           1,
						AvailableReplicas:  1,
						ObservedGeneration: 1,
						UpdatedReplicas:    1,
						ReadyReplicas:      1,
					}),
				)},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: jsonataTestSinkBinding(ctx, func(binding *sources.SinkBinding) {
					binding.Spec.Sink = sink2
				})},
			},
			WantEvents: []string{
				eventJsonataSinkBindingUpdated(),
			},
			WantErr: true,
		},
		{
			Name: "Reconcile sink binding ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithEventTransformSink(sink),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx),
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestSinkBinding(ctx, func(binding *sources.SinkBinding) {
					binding.Status = sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}
				}),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink),
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithJsonataSinkBindingStatus(sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
		},
		{
			Name: "Reconcile sink binding unset after being set",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx),
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestSinkBinding(ctx, func(binding *sources.SinkBinding) {
					binding.Status = sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}
				}),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  sources.SchemeGroupVersion.WithResource("sinkbindings"),
					},
					Name: jsonataTestSinkBinding(ctx).Name,
				},
			},
			WantEvents: []string{
				eventJsonataSinkBindingDeleted(),
			},
		},
		{
			Name: "Reconcile reply transformation",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyExpression(),
					WithEventTransformSink(sink),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx),
				jsonataTestSinkBinding(ctx, func(binding *sources.SinkBinding) {
					binding.Status = sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}
				}),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink),
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{}),
					WithJsonataSinkBindingStatus(sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
							SinkURI: sink.URI,
						},
					}),
				)},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: jsonataTestReplyDeployment(ctx, cw)},
			},
			WantCreates: []runtime.Object{
				jsonataReplyExpressionTestConfigMap(ctx),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataDeploymentUpdated(),
			},
			WantErr: true,
		},
		{
			Name: "Reconcile reply transformation ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyExpression(),
					WithEventTransformSink(sink),
				),
				jsonataTestReplyDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				jsonataReplyExpressionTestConfigMap(ctx),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx),
				jsonataTestSinkBinding(ctx, func(binding *sources.SinkBinding) {
					binding.Status = sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}
				}),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink),
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithJsonataSinkBindingStatus(sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
		},
		{
			Name: "Reconcile reply transformation discard",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyDiscard(),
					WithEventTransformSink(sink),
				),
				jsonataExpressionTestConfigMap(ctx),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx),
				jsonataTestSinkBinding(ctx, func(binding *sources.SinkBinding) {
					binding.Status = sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}
				}),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink),
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyDiscard(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataSinkBindingStatus(sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{}),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataTestReplyDiscardDeployment(ctx, cw),
			},
			WantEvents: []string{
				eventJsonataDeploymentCreated(),
			},
			WantErr: true, // skip key
		},
		{
			Name: "Reconcile reply transformation discard, ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyDiscard(),
					WithEventTransformSink(sink),
				),
				jsonataTestReplyDiscardDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				jsonataExpressionTestConfigMap(ctx),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				jsonataTestService(ctx),
				jsonataTestSinkBinding(ctx, func(binding *sources.SinkBinding) {
					binding.Status = sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}
				}),
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"}},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink),
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyDiscard(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithJsonataSinkBindingStatus(sources.SinkBindingStatus{
						SourceStatus: duckv1.SourceStatus{
							SinkURI: sink.URI,
							Status: duckv1.Status{
								Conditions: []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}},
							},
						},
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
		},
		{
			Name: "Reconcile initial loop, transport-encryption strict, endpoint ready, create certificate",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8443,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"},
					Data: map[string]string{
						"transport-encryption": "strict",
					},
				},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataCertificateStatus(cmapis.CertificateStatus{}),
					func(transform *v1alpha1.EventTransform) {
						transform.Status.JsonataTransformationStatus = nil
					},
				)},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx, func(service *corev1.Service) {
					service.Spec.Ports[0].Name = "https"
					service.Spec.Ports[0].AppProtocol = ptr.String("https")
					service.Spec.Ports[0].Port = 443
					service.Spec.Ports[0].TargetPort = intstr.FromInt32(8443)
				}),
				jsonataTestCertificate(ctx),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
				eventJsonataCertificateCreated(),
			},
			WantErr: true,
		},
		{
			Name: "Reconcile second loop, transport-encryption strict, endpoint ready, certificate ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
					// Disable HTTP Server in 'strict' mode.
					d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
						corev1.EnvVar{
							Name:  "DISABLE_HTTP_SERVER",
							Value: "true",
						},
					)

					// Switch probes to use HTTPS Scheme and Port.
					d.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Port = intstr.FromInt32(8443)
					d.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
					d.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt32(8443)
					d.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS

					// Inject TLS Cert and Key file paths.
					d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
						corev1.EnvVar{
							Name:  "HTTPS_CERT_PATH",
							Value: JsonataTLSCertPath,
						},
						corev1.EnvVar{
							Name:  "HTTPS_KEY_PATH",
							Value: JsonataTLSKeyPath,
						},
					)

					// Inject TLS Cert and Key secret volume.
					d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
						corev1.VolumeMount{
							Name:      JsonataTLSVolumeName,
							ReadOnly:  true,
							MountPath: JsonataTLSVolumePath,
						},
					)
					d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
						Name: JsonataTLSVolumeName,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: jsonataCertificateSecretName(NewEventTransform(testName, testNS)),
								Optional:   ptr.Bool(false),
							},
						},
					})

					d.Annotations[JsonataCertificateRevisionKey] = "1"
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8443,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"},
					Data: map[string]string{
						"transport-encryption": "strict",
					},
				},
				jsonataTestCertificate(ctx, func(certificate *cmapis.Certificate) {
					x := 1
					certificate.Status.Revision = &x
					certificate.Status.Conditions = append(certificate.Status.Conditions, cmapis.CertificateCondition{
						Type:   cmapis.CertificateConditionReady,
						Status: cmmeta.ConditionTrue,
					})
				}),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataCertificateStatus(cmapis.CertificateStatus{
						Revision: func() *int {
							x := 1
							return &x
						}(),
						Conditions: []cmapis.CertificateCondition{
							{Type: cmapis.CertificateConditionReady, Status: cmmeta.ConditionTrue},
						},
					}),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("https"),
							URL:  apis.HTTPS(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx, func(service *corev1.Service) {
					service.Spec.Ports[0].Name = "https"
					service.Spec.Ports[0].AppProtocol = ptr.String("https")
					service.Spec.Ports[0].Port = 443
					service.Spec.Ports[0].TargetPort = intstr.FromInt32(8443)
				}),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
			},
		},
		{
			Name: "Reconcile initial loop, transport-encryption permissive, endpoint ready, create certificate",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 80,
								},
								{
									Port: 443,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"},
					Data: map[string]string{
						"transport-encryption": "permissive",
					},
				},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataCertificateStatus(cmapis.CertificateStatus{}),
					func(transform *v1alpha1.EventTransform) {
						transform.Status.JsonataTransformationStatus = nil
					},
				)},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx, func(service *corev1.Service) {
					service.Spec.Ports = []corev1.ServicePort{
						{
							Name:        "https",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: ptr.String("https"),
							Port:        443,
							TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: 8443},
						},
						{
							Name:        "http",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: ptr.String("http"),
							Port:        80,
							TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
						},
					}
				}),
				jsonataTestCertificate(ctx),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
				eventJsonataCertificateCreated(),
			},
			WantErr: true,
		},
		{
			Name: "Reconcile second loop, transport-encryption permissive, endpoint ready, certificate ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
					// Inject TLS Cert and Key file paths.
					d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
						corev1.EnvVar{
							Name:  "HTTPS_CERT_PATH",
							Value: JsonataTLSCertPath,
						},
						corev1.EnvVar{
							Name:  "HTTPS_KEY_PATH",
							Value: JsonataTLSKeyPath,
						},
					)

					// Inject TLS Cert and Key secret volume.
					d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
						corev1.VolumeMount{
							Name:      JsonataTLSVolumeName,
							ReadOnly:  true,
							MountPath: JsonataTLSVolumePath,
						},
					)
					d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
						Name: JsonataTLSVolumeName,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: jsonataCertificateSecretName(NewEventTransform(testName, testNS)),
								Optional:   ptr.Bool(false),
							},
						},
					})

					d.Annotations[JsonataCertificateRevisionKey] = "1"
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
								{
									Port: 8443,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"},
					Data: map[string]string{
						"transport-encryption": "permissive",
					},
				},
				jsonataTestCertificate(ctx, func(certificate *cmapis.Certificate) {
					x := 1
					certificate.Status.Revision = &x
					certificate.Status.Conditions = append(certificate.Status.Conditions, cmapis.CertificateCondition{
						Type:   cmapis.CertificateConditionReady,
						Status: cmmeta.ConditionTrue,
					})
				}),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataCertificateStatus(cmapis.CertificateStatus{
						Revision: func() *int {
							x := 1
							return &x
						}(),
						Conditions: []cmapis.CertificateCondition{
							{Type: cmapis.CertificateConditionReady, Status: cmmeta.ConditionTrue},
						},
					}),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("https"),
							URL:  apis.HTTPS(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx, func(service *corev1.Service) {
					service.Spec.Ports = []corev1.ServicePort{
						{
							Name:        "https",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: ptr.String("https"),
							Port:        443,
							TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: 8443},
						},
						{
							Name:        "http",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: ptr.String("http"),
							Port:        80,
							TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
						},
					}
				}),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
			},
		},
		{
			Name: "Reconcile certificate updated, transport-encryption strict",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"},
					Data: map[string]string{
						"transport-encryption": "strict",
					},
				},
				jsonataTestCertificate(ctx, func(certificate *cmapis.Certificate) {
					certificate.Spec.SecretName = "foo"
					x := 1
					certificate.Status.Revision = &x
					certificate.Status.Conditions = append(certificate.Status.Conditions, cmapis.CertificateCondition{
						Type:   cmapis.CertificateConditionReady,
						Status: cmmeta.ConditionTrue,
					})
				}),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataCertificateStatus(cmapis.CertificateStatus{}),
					func(transform *v1alpha1.EventTransform) {
						transform.Status.JsonataTransformationStatus = nil
					},
				)},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: jsonataTestCertificate(ctx)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx, func(service *corev1.Service) {
					service.Spec.Ports[0].Name = "https"
					service.Spec.Ports[0].AppProtocol = ptr.String("https")
					service.Spec.Ports[0].Port = 443
					service.Spec.Ports[0].TargetPort = intstr.FromInt32(8443)
				}),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
				eventJsonataCertificateUpdated(),
			},
			WantErr: true, // skip key, waiting for certificate
		},
		{
			Name: "Reconcile certificate updated, transport-encryption permissive",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"},
					Data: map[string]string{
						"transport-encryption": "permissive",
					},
				},
				jsonataTestCertificate(ctx, func(certificate *cmapis.Certificate) {
					certificate.Spec.SecretName = "foo"
					x := 1
					certificate.Status.Revision = &x
					certificate.Status.Conditions = append(certificate.Status.Conditions, cmapis.CertificateCondition{
						Type:   cmapis.CertificateConditionReady,
						Status: cmmeta.ConditionTrue,
					})
				}),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataCertificateStatus(cmapis.CertificateStatus{}),
					func(transform *v1alpha1.EventTransform) {
						transform.Status.JsonataTransformationStatus = nil
					},
				)},
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: jsonataTestCertificate(ctx)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx, func(service *corev1.Service) {
					service.Spec.Ports = []corev1.ServicePort{
						{
							Name:        "https",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: ptr.String("https"),
							Port:        443,
							TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: 8443},
						},
						{
							Name:        "http",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: ptr.String("http"),
							Port:        80,
							TargetPort:  intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
						},
					}
				}),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
				eventJsonataCertificateUpdated(),
			},
			WantErr: true, // skip key, waiting for certificate
		},
		{
			Name: "Reconcile certificate deleted, transport-encryption turned off",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 8080,
								},
								{
									Port: 8443,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"},
					Data: map[string]string{
						"transport-encryption": "disabled",
					},
				},
				jsonataTestCertificate(ctx, func(certificate *cmapis.Certificate) {
					certificate.Spec.SecretName = "foo"
					x := 1
					certificate.Status.Revision = &x
					certificate.Status.Conditions = append(certificate.Status.Conditions, cmapis.CertificateCondition{
						Type:   cmapis.CertificateConditionReady,
						Status: cmmeta.ConditionTrue,
					})
				}),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithEventTransformAddresses(
						duckv1.Addressable{
							Name: ptr.String("http"),
							URL:  apis.HTTP(network.GetServiceHostname(jsonataTestService(ctx).Name, jsonataTestService(ctx).Namespace)),
						},
					),
				)},
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource:  cmapis.SchemeGroupVersion.WithResource("certificates"),
					},
					Name: jsonataTestCertificate(ctx).Name,
				},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
				eventJsonataCertificateDeleted(),
			},
		},
		{
			Name: "Reconcile second loop, transport-encryption strict, endpoints ports not ready, certificate ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
				jsonataTestDeployment(ctx, cw, func(d *appsv1.Deployment) {
					d.Status = appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}
					// Disable HTTP Server in 'strict' mode.
					d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
						corev1.EnvVar{
							Name:  "DISABLE_HTTP_SERVER",
							Value: "true",
						},
					)

					// Switch probes to use HTTPS Scheme and Port.
					d.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Port = intstr.FromInt32(8443)
					d.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
					d.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt32(8443)
					d.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS

					// Inject TLS Cert and Key file paths.
					d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
						corev1.EnvVar{
							Name:  "HTTPS_CERT_PATH",
							Value: JsonataTLSCertPath,
						},
						corev1.EnvVar{
							Name:  "HTTPS_KEY_PATH",
							Value: JsonataTLSKeyPath,
						},
					)

					// Inject TLS Cert and Key secret volume.
					d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
						corev1.VolumeMount{
							Name:      JsonataTLSVolumeName,
							ReadOnly:  true,
							MountPath: JsonataTLSVolumePath,
						},
					)
					d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
						Name: JsonataTLSVolumeName,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: jsonataCertificateSecretName(NewEventTransform(testName, testNS)),
								Optional:   ptr.Bool(false),
							},
						},
					})

					d.Annotations[JsonataCertificateRevisionKey] = "1"
				}),
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: jsonataTestService(ctx).Namespace,
						Name:      jsonataTestService(ctx).Name,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Ports: []corev1.EndpointPort{
								{
									Port: 80,
								},
							},
							Addresses: []corev1.EndpointAddress{
								{
									IP: "192.168.0.1",
								},
							},
						},
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: "config-features"},
					Data: map[string]string{
						"transport-encryption": "strict",
					},
				},
				jsonataTestCertificate(ctx, func(certificate *cmapis.Certificate) {
					x := 1
					certificate.Status.Revision = &x
					certificate.Status.Conditions = append(certificate.Status.Conditions, cmapis.CertificateCondition{
						Type:   cmapis.CertificateConditionReady,
						Status: cmmeta.ConditionTrue,
					})
				}),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataCertificateStatus(cmapis.CertificateStatus{
						Revision: func() *int {
							x := 1
							return &x
						}(),
						Conditions: []cmapis.CertificateCondition{
							{Type: cmapis.CertificateConditionReady, Status: cmmeta.ConditionTrue},
						},
					}),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{
						ObservedGeneration:  1,
						Replicas:            1,
						UpdatedReplicas:     1,
						ReadyReplicas:       1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
					}),
					WithJsonataWaitingForServiceEndpoints(),
				)},
			},
			WantCreates: []runtime.Object{
				jsonataExpressionTestConfigMap(ctx),
				jsonataTestService(ctx, func(service *corev1.Service) {
					service.Spec.Ports[0].Name = "https"
					service.Spec.Ports[0].AppProtocol = ptr.String("https")
					service.Spec.Ports[0].Port = 443
					service.Spec.Ports[0].TargetPort = intstr.FromInt32(8443)
				}),
			},
			WantEvents: []string{
				eventJsonataConfigMapCreated(),
				eventJsonataServiceCreated(),
			},
			WantErr: true, // skip key, waiting for endpoints
		},
	}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, watcher configmap.Watcher) controller.Reconciler {

		cmCertificatesListerAtomic := &atomic.Pointer[cmlisters.CertificateLister]{}
		cmCertificatesLister := listers.GetCertificateLister()
		cmCertificatesListerAtomic.Store(&cmCertificatesLister)

		r := &Reconciler{
			k8s:                        kubeclient.Get(ctx),
			client:                     eventingclient.Get(ctx),
			cmClient:                   cmclient.Get(ctx),
			jsonataConfigMapLister:     listers.GetConfigMapLister(),
			jsonataDeploymentsLister:   listers.GetDeploymentLister(),
			jsonataServiceLister:       listers.GetServiceLister(),
			jsonataEndpointLister:      listers.GetEndpointsLister(),
			jsonataSinkBindingLister:   listers.GetSinkBindingLister(),
			cmCertificateLister:        cmCertificatesListerAtomic,
			certificatesSecretLister:   listers.GetSecretLister(),
			trustBundleConfigMapLister: listers.GetConfigMapLister(),
			configWatcher:              cw,
		}

		store := feature.NewStore(logger.Named("config-store"))
		store.WatchConfigs(watcher)

		return eventtransform.NewReconciler(ctx,
			logger,
			fakeeventingclient.Get(ctx),
			listers.GetEventTransformLister(),
			controller.GetEventRecorder(ctx),
			r,
			controller.Options{
				ConfigStore: store,
			},
		)
	}, false, logger))
}

func jsonataReplyExpressionTestConfigMap(ctx context.Context, opts ...ConfigMapOption) *corev1.ConfigMap {
	cm := jsonataExpressionConfigMap(ctx, NewEventTransform(testName, testNS,
		WithEventTransformJsonataExpression(),
		WithEventTransformJsonataReplyExpression(),
	))
	for _, opt := range opts {
		opt(&cm)
	}
	return &cm
}

func jsonataExpressionTestConfigMap(ctx context.Context, opts ...ConfigMapOption) *corev1.ConfigMap {
	cm := jsonataExpressionConfigMap(ctx, NewEventTransform(testName, testNS,
		WithEventTransformJsonataExpression(),
	))
	for _, opt := range opts {
		opt(&cm)
	}
	return &cm
}

func jsonataTestDeployment(ctx context.Context, cw *reconcilersource.ConfigWatcher, opts ...DeploymentOption) *appsv1.Deployment {
	d := jsonataDeployment(ctx, false, cw, func() *corev1.ConfigMap {
		cm := jsonataExpressionConfigMap(ctx, NewEventTransform(testName, testNS,
			WithEventTransformJsonataExpression(),
		))
		return &cm
	}(), nil, NewEventTransform(testName, testNS,
		WithEventTransformJsonataExpression(),
	))

	for _, opt := range opts {
		opt(&d)
	}
	return &d
}

func jsonataTestReplyDeployment(ctx context.Context, cw *reconcilersource.ConfigWatcher, opts ...DeploymentOption) *appsv1.Deployment {
	d := jsonataDeployment(ctx, false, cw, func() *corev1.ConfigMap {
		cm := jsonataExpressionConfigMap(ctx, NewEventTransform(testName, testNS,
			WithEventTransformJsonataExpression(),
			WithEventTransformJsonataReplyExpression(),
		))
		return &cm
	}(), nil, NewEventTransform(testName, testNS,
		WithEventTransformJsonataExpression(),
		WithEventTransformJsonataReplyExpression(),
	))

	for _, opt := range opts {
		opt(&d)
	}
	return &d
}

func jsonataTestReplyDiscardDeployment(ctx context.Context, cw *reconcilersource.ConfigWatcher, opts ...DeploymentOption) *appsv1.Deployment {
	d := jsonataDeployment(ctx, false, cw, func() *corev1.ConfigMap {
		cm := jsonataExpressionConfigMap(ctx, NewEventTransform(testName, testNS,
			WithEventTransformJsonataExpression(),
			WithEventTransformJsonataReplyDiscard(),
		))
		return &cm
	}(), nil, NewEventTransform(testName, testNS,
		WithEventTransformJsonataExpression(),
		WithEventTransformJsonataReplyDiscard(),
	))

	for _, opt := range opts {
		opt(&d)
	}
	return &d
}

func jsonataTestService(ctx context.Context, opts ...ServiceOption) *corev1.Service {
	s := jsonataService(ctx, NewEventTransform(testName, testNS,
		WithEventTransformJsonataExpression(),
	))
	for _, opt := range opts {
		opt(&s)
	}
	return &s
}

func jsonataTestCertificate(ctx context.Context, opts ...func(certificate *cmapis.Certificate)) *cmapis.Certificate {
	s := jsonataCertificate(ctx, NewEventTransform(testName, testNS,
		WithEventTransformJsonataExpression(),
	))
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func jsonataTestSinkBinding(ctx context.Context, opts ...SinkBindingOption) *sources.SinkBinding {
	s := jsonataSinkBinding(ctx, NewEventTransform(testName, testNS,
		WithEventTransformJsonataExpression(),
		WithEventTransformSink(sink),
	))
	for _, opt := range opts {
		opt(&s)
	}
	return &s
}

func eventJsonataDeploymentCreated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataDeploymentCreated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataDeploymentUpdated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataDeploymentUpdated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataServiceCreated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataServiceCreated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataServiceUpdated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataServiceUpdated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataConfigMapCreated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataConfigMapCreated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataConfigMapUpdated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataConfigMapUpdated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataSinkBindingCreated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataSinkBindingCreated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataSinkBindingUpdated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataSinkBindingUpdated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataSinkBindingDeleted() string {
	return Eventf(corev1.EventTypeNormal, "JsonataSinkBindingDeleted", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataCertificateCreated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataCertificateCreated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataCertificateUpdated() string {
	return Eventf(corev1.EventTypeNormal, "JsonataCertificateUpdated", fmt.Sprintf("%s-%s", testName, "jsonata"))
}

func eventJsonataCertificateDeleted() string {
	return Eventf(corev1.EventTypeNormal, "JsonataCertificateDeleted", fmt.Sprintf("%s-%s", testName, "jsonata"))
}
