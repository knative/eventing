/*
Copyright 2018 The Knative Authors

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

package channel

import (
	"fmt"
	"testing"

	_ "knative.dev/pkg/system/testing"
)

const (
	referencesTestNamespace   = "test-namespace"
	referencesTestChannelName = "test-channel"
)

func TestChannelReference_String(t *testing.T) {
	ref := ChannelReference{
		Name:      referencesTestChannelName,
		Namespace: referencesTestNamespace,
	}
	expected := fmt.Sprintf("%s/%s", referencesTestNamespace, referencesTestChannelName)
	actual := ref.String()
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "ChannelReference", expected, actual)
	}
}

func TestParseChannel(t *testing.T) {
	c, err := ParseChannel("test-channel.test-namespace.svc.")
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if c.Name != "test-channel" {
		t.Errorf("Expected Name: test-channel. Got: %q", c.Name)
	}
	if c.Namespace != "test-namespace" {
		t.Errorf("Expected Name: test-namespace. Got: %q", c.Namespace)
	}
}
