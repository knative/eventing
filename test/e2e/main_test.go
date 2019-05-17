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
	"reflect"
	"runtime"
	"testing"

	"github.com/knative/eventing/test"
)

// channelTestMap indicates which test cases we want to run for a given CCP.
var channelTestMap = map[string][]func(t *testing.T){
	test.InMemoryProvisioner: {
		TestSingleBinaryEvent,
		TestSingleStructuredEvent,
		TestEventTransformation,
		TestChannelChain,
		TestDefaultBrokerWithManyTriggers,
		TestEventTransformationForTrigger,
	},
	test.GCPPubSubProvisioner: {
		TestSingleBinaryEvent,
		TestSingleStructuredEvent,
		TestEventTransformation,
		TestChannelChain,
		TestEventTransformationForTrigger,
	},
	test.NatssProvisioner: {
		TestSingleBinaryEvent,
		TestSingleStructuredEvent,
		TestEventTransformation,
		TestChannelChain,
		TestEventTransformationForTrigger,
	},
	test.KafkaProvisioner: {
		TestSingleBinaryEvent,
		TestSingleStructuredEvent,
		TestEventTransformation,
		TestChannelChain,
		TestEventTransformationForTrigger,
	},
}

func TestMain(t *testing.T) {
	// if the main test is not indicated to be run, skip it directly.
	if !test.EventingFlags.RunFromMain {
		t.Skip()
	}

	var provisioners = test.EventingFlags.Provisioners
	for _, provisioner := range provisioners {
		// set the current provisioner that is used to run the test cases
		ClusterChannelProvisionerToTest.Set(provisioner)
		for _, testFunc := range channelTestMap[provisioner] {
			funcName := runtime.FuncForPC(reflect.ValueOf(testFunc).Pointer()).Name()
			baseFuncName := GetBaseFuncName(funcName)
			t.Logf("Running %q with %q ClusterChannelProvisioner", baseFuncName, provisioner)
			t.Run(baseFuncName+"-"+provisioner, testFunc)
		}
	}
}
