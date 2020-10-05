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

package v1beta1

import "testing"

func TestSequenceGetStatus(t *testing.T) {
	r := &Sequence{
		Status: SequenceStatus{},
	}
	if got, want := r.GetStatus(), &r.Status.Status; got != want {
		t.Errorf("GetStatus=%v, want=%v", got, want)
	}
}

func TestSequenceKind(t *testing.T) {
	sequence := Sequence{}
	if sequence.GetGroupVersionKind().String() != "flows.knative.dev/v1beta1, Kind=Sequence" {
		t.Error("unexpected gvk:", sequence.GetGroupVersionKind())
	}
}

func TestSequence_GetGroupVersionKind(t *testing.T) {
	s := Sequence{}
	gvk := s.GetGroupVersionKind()
	if gvk.Kind != "Sequence" {
		t.Errorf("Should be Sequence.")
	}
}
