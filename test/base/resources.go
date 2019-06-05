/*
Copyright 2019 The Knative Authors

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

package base

// resources contains functions that construct Eventing CRs and other Kubernetes resources.

import (
	"fmt"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	sourcesv1alpha1 "github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	pkgTest "github.com/knative/pkg/test"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const eventingAPIVersion = "eventing.knative.dev/v1alpha1"
const sourcesAPIVersion = "sources.eventing.knative.dev/v1alpha1"

// clusterChannelProvisioner returns a ClusterChannelProvisioner for a given name.
func clusterChannelProvisioner(name string) *corev1.ObjectReference {
	if name == "" {
		return nil
	}
	return pkgTest.CoreV1ObjectReference("ClusterChannelProvisioner", eventingAPIVersion, name)
}

// channelRef returns an ObjectReference for a given Channel Name.
func channelRef(name string) *corev1.ObjectReference {
	return pkgTest.CoreV1ObjectReference("Channel", eventingAPIVersion, name)
}

// Channel returns a Channel with the specified provisioner.
func Channel(name, provisioner string) *eventingv1alpha1.Channel {
	return &eventingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: clusterChannelProvisioner(provisioner),
		},
	}
}

// WithSubscriberForSubscription returns an option that adds a Subscriber for the given Subscription.
func WithSubscriberForSubscription(name string) func(*eventingv1alpha1.Subscription) {
	return func(s *eventingv1alpha1.Subscription) {
		if name != "" {
			s.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{
				Ref: pkgTest.CoreV1ObjectReference("Service", "v1", name),
			}
		}
	}
}

// WithReply returns an options that adds a ReplyStrategy for the given Subscription.
func WithReply(name string) func(*eventingv1alpha1.Subscription) {
	return func(s *eventingv1alpha1.Subscription) {
		if name != "" {
			s.Spec.Reply = &eventingv1alpha1.ReplyStrategy{
				Channel: pkgTest.CoreV1ObjectReference("Channel", eventingAPIVersion, name),
			}
		}
	}
}

// Subscription returns a Subscription.
func Subscription(
	name,
	channelName string,
	options ...func(*eventingv1alpha1.Subscription),
) *eventingv1alpha1.Subscription {
	subscription := &eventingv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: eventingv1alpha1.SubscriptionSpec{
			Channel: *channelRef(channelName),
		},
	}
	for _, option := range options {
		option(subscription)
	}
	return subscription
}

// Broker returns a Broker.
func Broker(name, provisioner string) *eventingv1alpha1.Broker {
	return &eventingv1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: eventingv1alpha1.BrokerSpec{
			ChannelTemplate: &eventingv1alpha1.ChannelSpec{
				Provisioner: clusterChannelProvisioner(provisioner),
			},
		},
	}
}

// WithTriggerFilter returns an option that adds a TriggerFilter for the given Trigger.
func WithTriggerFilter(eventSource, eventType string) func(*eventingv1alpha1.Trigger) {
	return func(t *eventingv1alpha1.Trigger) {
		triggerFilter := &eventingv1alpha1.TriggerFilter{
			SourceAndType: &eventingv1alpha1.TriggerFilterSourceAndType{
				Type:   eventType,
				Source: eventSource,
			},
		}
		t.Spec.Filter = triggerFilter
	}
}

// WithBroker returns an option that adds a Broker for the given Trigger.
func WithBroker(brokerName string) func(*eventingv1alpha1.Trigger) {
	return func(t *eventingv1alpha1.Trigger) {
		t.Spec.Broker = brokerName
	}
}

// WithSubscriberRefForTrigger returns an option that adds a Subscriber Ref for the given Trigger.
func WithSubscriberRefForTrigger(name string) func(*eventingv1alpha1.Trigger) {
	return func(t *eventingv1alpha1.Trigger) {
		if name != "" {
			t.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{
				Ref: pkgTest.CoreV1ObjectReference("Service", "v1", name),
			}
		}
	}
}

// WithSubscriberURIForTrigger returns an option that adds a Subscriber URI for the given Trigger.
func WithSubscriberURIForTrigger(uri string) func(*eventingv1alpha1.Trigger) {
	return func(t *eventingv1alpha1.Trigger) {
		t.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{
			URI: &uri,
		}
	}
}

// Trigger returns a Trigger.
func Trigger(name string, options ...func(*eventingv1alpha1.Trigger)) *eventingv1alpha1.Trigger {
	trigger := &eventingv1alpha1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, option := range options {
		option(trigger)
	}
	return trigger
}

// WithSinkServiceForCronJobSource returns an option that adds a Kubernetes Service sink for the given CronJobSource.
func WithSinkServiceForCronJobSource(name string) func(*sourcesv1alpha1.CronJobSource) {
	return func(cjs *sourcesv1alpha1.CronJobSource) {
		cjs.Spec.Sink = pkgTest.CoreV1ObjectReference("Service", "v1", name)
	}
}

// WithServiceAccountForCronJobSource returns an option that adds a ServiceAccount for the given CronJobSource.
func WithServiceAccountForCronJobSource(saName string) func(*sourcesv1alpha1.CronJobSource) {
	return func(cjs *sourcesv1alpha1.CronJobSource) {
		cjs.Spec.ServiceAccountName = saName
	}
}

// CronJobSource returns a CronJob EventSource.
func CronJobSource(
	name,
	schedule,
	data string,
	options ...func(*sourcesv1alpha1.CronJobSource),
) *sourcesv1alpha1.CronJobSource {
	cronJobSource := &sourcesv1alpha1.CronJobSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: sourcesv1alpha1.CronJobSourceSpec{
			Schedule: schedule,
			Data:     data,
		},
	}
	for _, option := range options {
		option(cronJobSource)
	}
	return cronJobSource
}

// WithTemplateForContainerSource returns an option that adds a template for the given ContainerSource.
func WithTemplateForContainerSource(template *corev1.PodTemplateSpec) func(*sourcesv1alpha1.ContainerSource) {
	return func(cs *sourcesv1alpha1.ContainerSource) {
		cs.Spec.Template = template
	}
}

// WithSinkServiceForContainerSource returns an option that adds a Kubernetes Service sink for the given ContainerSource.
func WithSinkServiceForContainerSource(name string) func(*sourcesv1alpha1.ContainerSource) {
	return func(cs *sourcesv1alpha1.ContainerSource) {
		cs.Spec.Sink = pkgTest.CoreV1ObjectReference("Service", "v1", name)
	}
}

// ContainerSource returns a Container EventSource.
func ContainerSource(
	name string,
	options ...func(*sourcesv1alpha1.ContainerSource),
) *sourcesv1alpha1.ContainerSource {
	containerSource := &sourcesv1alpha1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, option := range options {
		option(containerSource)
	}
	return containerSource
}

// ContainerSourceBasicTemplate returns a basic template that can be used in ContainerSource.
func ContainerSourceBasicTemplate(
	name,
	namespace,
	imageName string,
	args []string,
) *corev1.PodTemplateSpec {
	envVars := []corev1.EnvVar{
		corev1.EnvVar{
			Name:  "POD_NAME",
			Value: name,
		},
		corev1.EnvVar{
			Name:  "POD_NAMESPACE",
			Value: namespace,
		},
	}

	podTemplateSpec := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				corev1.Container{
					Name:            imageName,
					Image:           pkgTest.ImagePath(imageName),
					ImagePullPolicy: corev1.PullAlways,
					Args:            args,
					Env:             envVars,
				},
			},
		},
	}
	return podTemplateSpec
}

// CloudEvent specifies the arguments for a CloudEvent sent by the sendevent
// binary.
type CloudEvent struct {
	ID       string
	Type     string
	Source   string
	Data     string
	Encoding string // binary or structured
}

// TypeAndSource specifies the type and source of an Event.
type TypeAndSource struct {
	Type   string
	Source string
}

// CloudEvent related constants.
const (
	CloudEventEncodingBinary     = "binary"
	CloudEventEncodingStructured = "structured"
	CloudEventDefaultEncoding    = CloudEventEncodingBinary
	CloudEventDefaultType        = "dev.knative.test.event"
)

// EventSenderPod creates a Pod that sends a single event to the given address.
func EventSenderPod(name string, sink string, event *CloudEvent) *corev1.Pod {
	if event.Encoding == "" {
		event.Encoding = CloudEventEncodingBinary
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "sendevent",
				Image:           pkgTest.ImagePath("sendevent"),
				ImagePullPolicy: corev1.PullAlways,
				Args: []string{
					"-event-id",
					event.ID,
					"-event-type",
					event.Type,
					"-source",
					event.Source,
					"-data",
					event.Data,
					"-encoding",
					event.Encoding,
					"-sink",
					sink,
				},
			}},
			//TODO restart on failure?
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

// EventLoggerPod creates a Pod that logs events received.
func EventLoggerPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "logevents",
				Image:           pkgTest.ImagePath("logevents"),
				ImagePullPolicy: corev1.PullAlways,
			}},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
}

// EventTransformationPod creates a Pod that transforms events received.
func EventTransformationPod(name string, event *CloudEvent) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "transformevents",
				Image:           pkgTest.ImagePath("transformevents"),
				ImagePullPolicy: corev1.PullAlways,
				Args: []string{
					"-event-type",
					event.Type,
					"-event-source",
					event.Source,
					"-event-data",
					event.Data,
				},
			}},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
}

// Service creates a Kubernetes Service with the given name, namespace, and
// selector. Port 8080 is assumed the target port.
func Service(name string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       80,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(8080),
			}},
		},
	}
}

// ServiceAccount creates a Kubernetes ServiceAccount with the given name and namespace.
func ServiceAccount(name, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// ClusterRoleBinding creates a Kubernetes ClusterRoleBinding with the given ServiceAccount name, ClusterRole name and namespace.
func ClusterRoleBinding(saName, crName, namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-admin", saName, namespace),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     crName,
			APIGroup: rbacv1.SchemeGroupVersion.Group,
		},
	}
}
