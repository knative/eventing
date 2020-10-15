/*
Copyright 2020 The Knative Authors

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

package tracing

import (
	"reflect"
	"testing"

	"go.opencensus.io/trace"

	"k8s.io/apimachinery/pkg/types"
)

func TestBrokerMessagingDestination(t *testing.T) {
	got := BrokerMessagingDestination(types.NamespacedName{
		Namespace: "brokerns",
		Name:      "brokername",
	})
	want := "broker:brokername.brokerns"
	if want != got {
		t.Errorf("unexpected messaging destination: want %q, got %q", want, got)
	}
}

func TestTriggerMessagingDestination(t *testing.T) {
	got := TriggerMessagingDestination(types.NamespacedName{
		Namespace: "triggerns",
		Name:      "triggername",
	})
	want := "trigger:triggername.triggerns"
	if want != got {
		t.Errorf("unexpected messaging destination: want %q, got %q", want, got)
	}
}

func TestMessagingMessageIDAttribute(t *testing.T) {
	tests := []struct {
		name string
		ID   string
		want trace.Attribute
	}{
		{
			name: "foo",
			ID:   "foo",
			want: trace.StringAttribute("messaging.message_id", "foo"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MessagingMessageIDAttribute(tt.ID); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MessagingMessageIDAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBrokerMessagingDestinationAttribute(t *testing.T) {

	tests := []struct {
		name          string
		namespaceName types.NamespacedName
		want          trace.Attribute
	}{
		{
			name: "foo",
			namespaceName: types.NamespacedName{
				Namespace: "default",
				Name:      "foo",
			},

			want: trace.StringAttribute("messaging.destination", "broker:foo.default"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BrokerMessagingDestinationAttribute(tt.namespaceName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BrokerMessagingDestinationAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTriggerMessagingDestinationAttribute(t *testing.T) {

	tests := []struct {
		name          string
		namespaceName types.NamespacedName
		want          trace.Attribute
	}{
		{
			name: "foo",
			namespaceName: types.NamespacedName{
				Namespace: "default",
				Name:      "foo",
			},

			want: trace.StringAttribute("messaging.destination", "trigger:foo.default"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TriggerMessagingDestinationAttribute(tt.namespaceName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TriggerMessagingDestinationAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}
