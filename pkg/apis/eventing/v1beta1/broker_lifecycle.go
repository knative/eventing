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

import (
	"knative.dev/pkg/apis"
)

var brokerCondSet = apis.NewLivingConditionSet(BrokerConditionAddressable)

const (
	BrokerConditionReady                          = apis.ConditionReady
	BrokerConditionAddressable apis.ConditionType = "Addressable"
)

// GetTopLevelCondition returns the top level Condition.
func (bs *BrokerStatus) GetTopLevelCondition() *apis.Condition {
	return brokerCondSet.Manage(bs).GetTopLevelCondition()
}

// SetAddress makes this Broker addressable by setting the URI. It also
// sets the BrokerConditionAddressable to true.
func (bs *BrokerStatus) SetAddress(url *apis.URL) {
	bs.Address.URL = url
	if url != nil {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionAddressable)
	} else {
		brokerCondSet.Manage(bs).MarkFalse(BrokerConditionAddressable, "nil URL", "URL is nil")
	}
}
