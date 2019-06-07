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
	"github.com/knative/eventing/contrib/natss/pkg/apis/messaging/v1alpha1"
	"github.com/knative/pkg/kmeta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	serviceName    = "my-test-service"
	ncName         = "my-test-nc"
	testNS         = "my-test-ns"
	dispatcherNS   = "dispatcher-namespace"
	dispatcherName = "dispatcher-name"
)

func TestMakeExternalServiceAddress(t *testing.T) {
	if want, got := "my-test-service.my-test-ns.svc.cluster.local", MakeExternalServiceAddress(testNS, serviceName); want != got {
		t.Errorf("Want: %q got %q", want, got)
	}
}

func TestMakeChannelServiceAddress(t *testing.T) {
	if want, got := "my-test-nc-kn-channel", MakeChannelServiceName(ncName); want != got {
		t.Errorf("Want: %q got %q", want, got)
	}
}

func TestMakeService(t *testing.T) {
	imc := &v1alpha1.NatssChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ncName,
			Namespace: testNS,
		},
	}
	want := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kn-channel", ncName),
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
					Name:     portName,
					Protocol: corev1.ProtocolTCP,
					Port:     portNumber,
				},
			},
		},
	}

	got, err := MakeK8sService(imc)
	if err != nil {
		t.Fatalf("Failed to create new service: %s", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected condition (-want, +got) = %v", diff)
	}
}

func TestMakeServiceWithExternal(t *testing.T) {
	imc := &v1alpha1.NatssChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ncName,
			Namespace: testNS,
		},
	}
	want := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kn-channel", ncName),
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

	got, err := MakeK8sService(imc, ExternalService(dispatcherNS, dispatcherName))
	if err != nil {
		t.Fatalf("Failed to create new service: %s", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected condition (-want, +got) = %v", diff)
	}
}

func TestMakeServiceWithFailingOption(t *testing.T) {
	imc := &v1alpha1.NatssChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ncName,
			Namespace: testNS,
		},
	}
	_, err := MakeK8sService(imc, func(svc *corev1.Service) error { return errors.New("test-induced failure") })
	if err == nil {
		t.Fatalf("Expcted error from new service but got none")
	}
}
