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

package resources

import "testing"

func TestMakeSubscriptionCRD(t *testing.T) {
	brokerName := "default"
	channelType := "ingress"

	nonCRD := NonCRDBrokerChannelName(brokerName, channelType)
	crd := BrokerChannelName(brokerName, channelType)
	if nonCRD == crd {
		t.Fatalf("NonCRD and CRD Channel names should be different: %q == %q", nonCRD, crd)
	}
}
