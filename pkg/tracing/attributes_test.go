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
	"testing"

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
