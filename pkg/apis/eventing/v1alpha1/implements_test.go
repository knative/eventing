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
package v1alpha1

import (
	"testing"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis/duck"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func TestTypesImplements(t *testing.T) {
	testCases := []struct {
		instance interface{}
		iface    duck.Implementable
	}{
		// Channel
		{instance: &Channel{}, iface: &duckv1beta1.Conditions{}},
		{instance: &Channel{}, iface: &eventingduck.Subscribable{}},
		{instance: &Channel{}, iface: &duckv1alpha1.Addressable{}},
		// ClusterChannelProvisioner
		{instance: &ClusterChannelProvisioner{}, iface: &duckv1beta1.Conditions{}},
		// Subscription
		{instance: &Subscription{}, iface: &duckv1beta1.Conditions{}},
	}
	for _, tc := range testCases {
		if err := duck.VerifyType(tc.instance, tc.iface); err != nil {
			t.Error(err)
		}
	}
}
