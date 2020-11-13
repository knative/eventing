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

package resources

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/network"
)

const (
	serviceName    = "my-test-service"
	imcName        = "my-test-imc"
	testNS         = "my-test-ns"
	dispatcherNS   = "dispatcher-namespace"
	dispatcherName = "dispatcher-name"
)

func TestCreateExternalServiceAddress(t *testing.T) {
	if want, got := "my-test-service.my-test-ns.svc.cluster.local", network.GetServiceHostname(serviceName, testNS); want != got {
		t.Errorf("Want: %q got %q", want, got)
	}
}

func TestCreateChannelServiceAddress(t *testing.T) {
	if want, got := "my-test-imc-kn-channel", CreateChannelServiceName(imcName); want != got {
		t.Errorf("Want: %q got %q", want, got)
	}
}

func TestNewK8sService(t *testing.T) {
	imc := &v1.InMemoryChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imcName,
			Namespace: testNS,
		},
	}
	want := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kn-channel", imcName),
			Namespace: testNS,
			Labels: map[string]string{
				MessagingRoleLabel: MessagingRole,
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(imc),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     PortName,
					Protocol: corev1.ProtocolTCP,
					Port:     PortNumber,
				},
			},
		},
	}

	got, err := NewK8sService(imc)
	if err != nil {
		t.Fatal("Failed to create new service:", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected condition (-want, +got) =", diff)
	}
}

func TestNewK8sServiceWithExternal(t *testing.T) {
	imc := &v1.InMemoryChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imcName,
			Namespace: testNS,
		},
	}
	want := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kn-channel", imcName),
			Namespace: testNS,
			Labels: map[string]string{
				MessagingRoleLabel: MessagingRole,
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(imc),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "dispatcher-name.dispatcher-namespace.svc.cluster.local",
		},
	}

	got, err := NewK8sService(imc, ExternalService(dispatcherNS, dispatcherName))
	if err != nil {
		t.Fatal("Failed to create new service:", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected condition (-want, +got) =", diff)
	}
}

func TestNewK8sServiceWithFailingOption(t *testing.T) {
	imc := &v1.InMemoryChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imcName,
			Namespace: testNS,
		},
	}
	_, err := NewK8sService(imc, func(svc *corev1.Service) error { return errors.New("test-induced failure") })
	if err == nil {
		t.Fatalf("Expected error from new service but got none")
	}
}
