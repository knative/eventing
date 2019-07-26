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
	"testing"

	duckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestBrokerChannelName(t *testing.T) {
	// Any changes to this name are breaking changes, this test is here so that changes can't be
	// made by accident.
	expected := "default-kn-ingress"
	if actual := BrokerChannelName("default", "ingress"); actual != expected {
		t.Errorf("expected %q, actual %q", expected, actual)
	}
}

func TestNewChannel(t *testing.T) {
	testCases := map[string]struct {
		channelTemplate duckv1alpha1.ChannelTemplateSpec
		expectError     bool
	}{
		"InMemoryChannel": {
			channelTemplate: duckv1alpha1.ChannelTemplateSpec{
				TypeMeta: v1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1alpha1",
					Kind:       "InMemoryChannel",
				},
			},
		},
		"KafkaChannel": {
			channelTemplate: duckv1alpha1.ChannelTemplateSpec{
				TypeMeta: v1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1alpha1",
					Kind:       "KafkaChannel",
				},
			},
		},
		"Bad raw extension": {
			channelTemplate: duckv1alpha1.ChannelTemplateSpec{
				TypeMeta: v1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1alpha1",
					Kind:       "InMemoryChannel",
				},
				Spec: &runtime.RawExtension{
					Raw: []byte("hello world"),
				},
			},
			expectError: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			b := &v1alpha1.Broker{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "brokers-namespace",
					Name:      "my-broker",
					UID:       "1234",
				},
				Spec: v1alpha1.BrokerSpec{
					ChannelTemplate: &tc.channelTemplate,
				},
			}
			labels := map[string]string{"key": "value"}
			c, err := NewChannel("ingress", b, labels)
			if err != nil {
				if !tc.expectError {
					t.Fatalf("Unexpected error calling NewChannel: %v", err)
				}
				return
			} else if tc.expectError {
				t.Fatalf("Expected an error calling NewChannel, actually nil")
			}

			if api := c.Object["apiVersion"]; api != tc.channelTemplate.APIVersion {
				t.Errorf("Expected APIVersion %q, actually %q", tc.channelTemplate.APIVersion, api)
			}
			if kind := c.Object["kind"]; kind != tc.channelTemplate.Kind {
				t.Errorf("Expected Kind %q, actually %q", tc.channelTemplate.Kind, kind)
			}

			md := c.Object["metadata"].(map[string]interface{})
			assertSoleOwner(t, b, c)
			if md["namespace"] != b.Namespace {
				t.Errorf("expected namespace %q, actually %q", b.Namespace, md["namespace"])
			}
			if name := md["name"]; name != "my-broker-kn-ingress" {
				t.Errorf("Expected name %q, actually %q", "my-broker-kn-ingress", name)
			}
			if l := md["labels"].(map[string]interface{}); len(l) != len(labels) {
				t.Errorf("Expected labels %q, actually %q", labels, l)
			} else {
				for k, v := range labels {
					if l[k] != v {
						t.Errorf("Expected labels %q, actually %q", labels, l)
					}
				}
			}
		})
	}
}

func assertSoleOwner(t *testing.T, owner v1.Object, owned *unstructured.Unstructured) {
	md := owned.Object["metadata"].(map[string]interface{})
	owners := md["ownerReferences"].([]interface{})
	if len(owners) != 1 {
		t.Errorf("Expected 1 owner, actually %d", len(owners))
	}
	o := owners[0].(map[string]interface{})
	if uid := o["uid"]; uid != string(owner.GetUID()) {
		t.Errorf("Expected UID %q, actually %q", owner.GetUID(), uid)
	}
	if name := o["name"]; name != owner.GetName() {
		t.Errorf("Expected name %q, actually %q", owner.GetName(), name)
	}
	if !o["controller"].(bool) {
		t.Error("Expected controller true, actually false")
	}
}
