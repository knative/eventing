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

package resources

import (
	"testing"
)

func TestBrokerChannelName(t *testing.T) {
	// Any changes to this name are breaking changes, this test is here so that changes can't be
	// made by accident.
	expected := "default-kne-ingress"
	if actual := BrokerChannelName("default", "ingress"); actual != expected {
		t.Errorf("expected %q, actual %q", expected, actual)
	}
}
