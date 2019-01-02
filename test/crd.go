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
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Route returns a Route object in namespace
func Route(name string, namespace string, configName string) *servingv1alpha1.Route {
	return &servingv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: servingv1alpha1.RouteSpec{
			Traffic: []servingv1alpha1.TrafficTarget{
				{
					ConfigurationName: configName,
					Percent:           100,
				},
			},
		},
	}
}

// Configuration returns a Configuration object in namespace with the name names.Config
// that uses the image specified by imagePath.
func Configuration(name string, namespace string, imagePath string) *servingv1alpha1.Configuration {
	return &servingv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: servingv1alpha1.ConfigurationSpec{
			RevisionTemplate: servingv1alpha1.RevisionTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"knative.dev/type": "container"},
				},
				Spec: servingv1alpha1.RevisionSpec{
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

// ClusterRoleBinding returns ClusterRoleBinding for given subject and role
func ClusterRoleBinding(name string, namespace string, serviceAccount string, role string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     role,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

// ClusterChannelProvisioner returns a ClusterChannelProvisioner for a given name
func ClusterChannelProvisioner(name string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:       "ClusterChannelProvisioner",
		APIVersion: "eventing.knative.dev/v1alpha1",
		Name:       name,
	}
}

// ChannelRef returns an ObjectReference for a given Channel Name
func ChannelRef(name string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:       "Channel",
		APIVersion: "eventing.knative.dev/v1alpha1",
		Name:       name,
	}
}

// Channel returns a Channel with the specified provisioner
func Channel(name string, namespace string, provisioner *corev1.ObjectReference) *v1alpha1.Channel {
	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ChannelSpec{
			Provisioner: provisioner,
		},
	}
}

// KubernetesEventSource returns a KubernetesEventSource sinking to specified channel
func KubernetesEventSource(name string, namespace string, targetNamespace string, serviceAccount string, channel *corev1.ObjectReference) *sourcesv1alpha1.KubernetesEventSource {
	return &sourcesv1alpha1.KubernetesEventSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: sourcesv1alpha1.KubernetesEventSourceSpec{
			Namespace:          targetNamespace,
			ServiceAccountName: serviceAccount,
			Sink:               channel,
		},
	}
}

// SubscriberSpecForRoute returns a SubscriberSpec for a given Knative Service.
func SubscriberSpecForRoute(name string) *v1alpha1.SubscriberSpec {
	return &v1alpha1.SubscriberSpec{
		Ref: &corev1.ObjectReference{
			Kind:       "Route",
			APIVersion: "serving.knative.dev/v1alpha1",
			Name:       name,
		},
	}
}

// Subscription returns a Subscription
func Subscription(name string, namespace string, channel *corev1.ObjectReference, subscriber *v1alpha1.SubscriberSpec, reply *v1alpha1.ReplyStrategy) *v1alpha1.Subscription {
	return &v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel:    *channel,
			Subscriber: subscriber,
			Reply:      reply,
		},
	}
}

// NGinxPod returns nginx pod defined in given namespace
func NGinxPod(namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "nginx",
			Namespace:   namespace,
			Annotations: map[string]string{"sidecar.istio.io/inject": "true"},
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
