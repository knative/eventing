/*
Copyright 2022 The Knative Authors

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

package main

import (
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
)

func TestSchemaRegistrations(t *testing.T) {
	sb := sourcesv1.SinkBinding{}
	_, _, err := scheme.Scheme.ObjectKinds(&sb)
	if err != nil {
		t.Errorf("Schemas should have been registered: %v", err)
	}
}
