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

package v1alpha1_test

import (
	corev1 "k8s.io/api/core/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"testing"

	"knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
)

func TestMarkDeprecated_empty(t *testing.T) {
	s := &duckv1.Status{}
	d := &v1alpha1.Deprecated{}

	d.MarkDeprecated(s, "Dep", "unit test")

	ds := s.GetCondition(v1alpha1.StatusConditionTypeDeprecated)
	if ds == nil {
		t.Errorf("Expected to get a Deprecated type condition, but found none.")
		return
	}
	if ds.Status != corev1.ConditionTrue {
		t.Errorf("Expected Deprecated status to be true, got %s.", ds.Status)
	}
	if ds.Reason != "Dep" {
		t.Errorf("Expected Deprecated reason to be 'Dep', got %s.", ds.Reason)
	}
	if ds.Message != "unit test" {
		t.Errorf("Expected Deprecated message to be 'unit test', got %s.", ds.Message)
	}
}

func TestMarkDeprecated_set(t *testing.T) {
	s := &duckv1.Status{}
	d := &v1alpha1.Deprecated{}

	d.MarkDeprecated(s, "Pre", "prefill")
	d.MarkDeprecated(s, "Dep", "unit test")

	ds := s.GetCondition(v1alpha1.StatusConditionTypeDeprecated)
	if ds == nil {
		t.Errorf("Expected to get a Deprecated type condition, but found none.")
		return
	}
	if ds.Status != corev1.ConditionTrue {
		t.Errorf("Expected Deprecated status to be true, got %s.", ds.Status)
	}
	if ds.Reason != "Dep" {
		t.Errorf("Expected Deprecated reason to be 'Dep', got %s.", ds.Reason)
	}
	if ds.Message != "unit test" {
		t.Errorf("Expected Deprecated message to be 'unit test', got %s.", ds.Message)
	}
}
