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

package common

import (
	pkgTest "github.com/knative/pkg/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

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
func EventSenderPod(name string, namespace string, sink string, event *CloudEvent) *corev1.Pod {
	if event.Encoding == "" {
		event.Encoding = CloudEventEncodingBinary
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
func EventLoggerPod(name string, namespace string, selector map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    selector,
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
func EventTransformationPod(name string, namespace string, selector map[string]string, event *CloudEvent) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    selector,
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
func Service(name string, namespace string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
