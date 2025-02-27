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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	sources "knative.dev/eventing/pkg/apis/sources/v1"
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
		},
		{
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		},
		{
			Name: "Object not found",
			Key:  testKey,
		},
		{
			Name: "Reconcile initial loop",
			Key:  testKey,
			Objects: []runtime.Object{
				NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatusSkipSinkBinding(),
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
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformJsonataExpression(),
					WithJsonataEventTransformInitializeStatusSkipSinkBinding(),
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
					WithJsonataSinkBindingStatus(sources.SinkBindingStatus{}),
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
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink2),
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
					WithJsonataSinkBindingStatus(sources.SinkBindingStatus{}),
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
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink),
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyExpression(),
					WithJsonataEventTransformInitializeStatus(),
					WithJsonataDeploymentStatus(appsv1.DeploymentStatus{}),
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
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewEventTransform(testName, testNS,
					WithEventTransformSink(sink),
					WithEventTransformJsonataExpression(),
					WithEventTransformJsonataReplyDiscard(),
					WithJsonataEventTransformInitializeStatus(),
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
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, watcher configmap.Watcher) controller.Reconciler {

		r := &Reconciler{
			k8s:                      kubeclient.Get(ctx),
			client:                   eventingclient.Get(ctx),
			jsonataConfigMapLister:   listers.GetConfigMapLister(),
			jsonataDeploymentsLister: listers.GetDeploymentLister(),
			jsonataServiceLister:     listers.GetServiceLister(),
			jsonataEndpointLister:    listers.GetEndpointsLister(),
			jsonataSinkBindingLister: listers.GetSinkBindingLister(),
			configWatcher:            cw,
		}

		return eventtransform.NewReconciler(ctx,
			logger,
			fakeeventingclient.Get(ctx),
			listers.GetEventTransformLister(),
			controller.GetEventRecorder(ctx),
			r,
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
	d := jsonataDeployment(ctx,
		cw,
		func() *corev1.ConfigMap {
			cm := jsonataExpressionConfigMap(ctx, NewEventTransform(testName, testNS,
				WithEventTransformJsonataExpression(),
			))
			return &cm
		}(),
		NewEventTransform(testName, testNS,
			WithEventTransformJsonataExpression(),
		),
	)

	for _, opt := range opts {
		opt(&d)
	}
	return &d
}

func jsonataTestReplyDeployment(ctx context.Context, cw *reconcilersource.ConfigWatcher, opts ...DeploymentOption) *appsv1.Deployment {
	d := jsonataDeployment(ctx,
		cw,
		func() *corev1.ConfigMap {
			cm := jsonataExpressionConfigMap(ctx, NewEventTransform(testName, testNS,
				WithEventTransformJsonataExpression(),
				WithEventTransformJsonataReplyExpression(),
			))
			return &cm
		}(),
		NewEventTransform(testName, testNS,
			WithEventTransformJsonataExpression(),
			WithEventTransformJsonataReplyExpression(),
		),
	)

	for _, opt := range opts {
		opt(&d)
	}
	return &d
}

func jsonataTestReplyDiscardDeployment(ctx context.Context, cw *reconcilersource.ConfigWatcher, opts ...DeploymentOption) *appsv1.Deployment {
	d := jsonataDeployment(ctx,
		cw,
		func() *corev1.ConfigMap {
			cm := jsonataExpressionConfigMap(ctx, NewEventTransform(testName, testNS,
				WithEventTransformJsonataExpression(),
				WithEventTransformJsonataReplyDiscard(),
			))
			return &cm
		}(),
		NewEventTransform(testName, testNS,
			WithEventTransformJsonataExpression(),
			WithEventTransformJsonataReplyDiscard(),
		),
	)

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
