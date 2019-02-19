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

package v1alpha1

import (
	"testing"
)

// No-op test because method does nothing.
func TestBrokerValidation(t *testing.T) {
	b := Broker{}
	_ = b.Validate()
}

// No-op test because method does nothing.
func TestBrokerSpecValidation(t *testing.T) {
	bs := BrokerSpec{}
	_ = bs.Validate()
}

// No-op test because method does nothing.
func TestBrokerImmutableFields(t *testing.T) {
	original := &Broker{}
	current := &Broker{}
	_ = current.CheckImmutableFields(original)
}
