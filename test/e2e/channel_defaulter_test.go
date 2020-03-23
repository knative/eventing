// +build e2e

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

package e2e

import (
	"testing"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/test/e2e/helpers"
)

// TestChannelClusterDefaulter tests a cluster defaulted channel can be created with the template specified through configmap.
func TestChannelClusterDefaulter(t *testing.T) {
	// Since these tests already get run as part of the ChannelBasedBroker, there's no point
	// rerunning them again for MT broker. Or if want to, we then have to create the namespace
	// with a label that prevents the Broker from (and hence a trigger channel for it being
	// created). Either case, seems silly to run the tests twice.
	if brokerClass == eventing.MTChannelBrokerClassValue {
		t.Skip("Not double running tests for MT Broker")
	}
	helpers.ChannelClusterDefaulterTestHelper(t, channelTestRunner)
}

// TestChannelNamespaceDefaulter tests a namespace defaulted channel can be created with the template specified through configmap.
func TestChannelNamespaceDefaulter(t *testing.T) {
	// Since these tests already get run as part of the ChannelBasedBroker, there's no point
	// rerunning them again for MT broker. Or if want to, we then have to create the namespace
	// with a label that prevents the Broker from (and hence a trigger channel for it being
	// created). Either case, seems silly to run the tests twice.
	if brokerClass == eventing.MTChannelBrokerClassValue {
		t.Skip("Not double running tests for MT Broker")
	}
	helpers.ChannelNamespaceDefaulterTestHelper(t, channelTestRunner)
}
