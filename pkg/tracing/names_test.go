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

package tracing

import "testing"

func TestBrokerIngressName(t *testing.T) {
	const testNS = "test-namespace"
	const broker = "my-broker"
	args := BrokerIngressNameArgs{
		Namespace:  testNS,
		BrokerName: broker,
	}
	if got, want := BrokerIngressName(args), "my-broker-broker.test-namespace"; got != want {
		t.Errorf("BrokerIngressName = %q, want %q", got, want)
	}
}

func TestBrokerFilterName(t *testing.T) {
	const testNS = "test-namespace"
	const broker = "my-broker"
	args := BrokerFilterNameArgs{
		Namespace:  testNS,
		BrokerName: broker,
	}
	if got, want := BrokerFilterName(args), "my-broker-broker-filter.test-namespace"; got != want {
		t.Errorf("BrokerFilterName = %q, want %q", got, want)
	}
}
