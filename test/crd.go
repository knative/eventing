/*
Copyright 2018 The Knative Authors
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

package test

// crd contains functions that construct boilerplate CRD definitions.

import (
	"fmt"

	channelsV1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsV1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	flowsV1alpha1 "github.com/knative/eventing/pkg/apis/flows/v1alpha1"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacV1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Route returns a Route object in namespace
func Route(name string, namespace string, configName string) *v1alpha1.Route {
	return &v1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.RouteSpec{
			Traffic: []v1alpha1.TrafficTarget{
				v1alpha1.TrafficTarget{
					ConfigurationName: configName,
					Percent:           100,
				},
			},
		},
	}
}

// Configuration returns a Configuration object in namespace with the name names.Config
// that uses the image specifed by imagePath.
func Configuration(name string, namespace string, imagePath string) *v1alpha1.Configuration {
	return &v1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ConfigurationSpec{
			RevisionTemplate: v1alpha1.RevisionTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"knative.dev/type": "container"},
				},
				Spec: v1alpha1.RevisionSpec{
					Container: corev1.Container{
						Image: imagePath,
					},
				},
			},
		},
	}
}

// ServiceAccount returns ServiceAccount object in given namespace
func ServiceAccount(name string, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// ClusterRoleBinding create ClusterRoleBinding for given subject and role
func ClusterRoleBinding(name string, namespace string, serviceAccount string, role string) *rbacV1beta1.ClusterRoleBinding {
	return &rbacV1beta1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacV1beta1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacV1beta1.RoleRef{
			Kind:     "ClusterRole",
			Name:     role,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

// ClusterBus returns ClusterBus object with given name and imagePath
func ClusterBus(name string, namespace string, imagePath string) *channelsV1alpha1.ClusterBus {
	return &channelsV1alpha1.ClusterBus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: channelsV1alpha1.ClusterBusSpec{
			Dispatcher: corev1.Container{
				Name:  "dispatcher",
				Image: imagePath,
				Args:  []string{"-logtostderr", "-stderrthreshold", "INFO"},
			},
		},
	}
}

// EventSource returns EventSource object using the given image paths
func EventSource(eventSource string, namespace string, eventSourceImagePath string, receiverAdapterImagePath string) *feedsV1alpha1.EventSource {
	return &feedsV1alpha1.EventSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eventSource,
			Namespace: namespace,
		},
		Spec: feedsV1alpha1.EventSourceSpec{
			CommonEventSourceSpec: feedsV1alpha1.CommonEventSourceSpec{
				Source: eventSource,
				Image:  eventSourceImagePath,
				Parameters: &runtime.RawExtension{
					Raw: []byte(fmt.Sprintf(`{"image": "%s"}`, receiverAdapterImagePath)),
				},
			},
		},
	}
}

// EventType returns an EventType object referencing given eventSource
func EventType(name string, namespace string, eventSource string) *feedsV1alpha1.EventType {
	return &feedsV1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: feedsV1alpha1.EventTypeSpec{
			CommonEventTypeSpec: feedsV1alpha1.CommonEventTypeSpec{
				Description: "subscription for receiving k8s cluster events",
			},
			EventSource: eventSource,
		},
	}
}

// Flow will return Flow object with given parameters
func Flow(flowName string, namespace string, serviceAccount string, eventType string, eventSource string, routeName string, testNamespace string) *flowsV1alpha1.Flow {
	return &flowsV1alpha1.Flow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      flowName,
			Namespace: namespace,
		},
		Spec: flowsV1alpha1.FlowSpec{
			ServiceAccountName: serviceAccount,
			Trigger: flowsV1alpha1.EventTrigger{
				EventType: eventType,
				Resource:  "k8sevents/receiveevent",
				Service:   eventSource,
				Parameters: &runtime.RawExtension{
					Raw: []byte(fmt.Sprintf(`{"namespace": "%s"}`, testNamespace)),
				},
			},
			Action: flowsV1alpha1.FlowAction{
				Target: &corev1.ObjectReference{
					Kind:       "Route",
					APIVersion: "serving.knative.dev/v1alpha1",
					Name:       routeName,
				},
			},
		},
	}
}

// NGinxPod returns nginx pod defined in given namespace
func NGinxPod(namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.7.9",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				},
			},
		},
	}
}
