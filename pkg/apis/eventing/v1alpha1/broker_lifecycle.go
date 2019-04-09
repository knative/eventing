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

import duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"

var brokerCondSet = duckv1alpha1.NewLivingConditionSet(
	BrokerConditionIngress,
	BrokerConditionTriggerChannel,
	BrokerConditionIngressChannel,
	BrokerConditionFilter,
	BrokerConditionAddressable,
	BrokerConditionIngressSubscription,
)

const (
	BrokerConditionReady                              = duckv1alpha1.ConditionReady
	BrokerConditionIngress duckv1alpha1.ConditionType = "IngressReady"

	BrokerConditionTriggerChannel duckv1alpha1.ConditionType = "TriggerChannelReady"

	BrokerConditionIngressChannel duckv1alpha1.ConditionType = "IngressChannelReady"

	BrokerConditionIngressSubscription duckv1alpha1.ConditionType = "IngressSubscriptionReady"

	BrokerConditionFilter duckv1alpha1.ConditionType = "FilterReady"

	BrokerConditionAddressable duckv1alpha1.ConditionType = "Addressable"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (bs *BrokerStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return brokerCondSet.Manage(bs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (bs *BrokerStatus) IsReady() bool {
	return brokerCondSet.Manage(bs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (bs *BrokerStatus) InitializeConditions() {
	brokerCondSet.Manage(bs).InitializeConditions()
}

func (bs *BrokerStatus) MarkIngressReady() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionIngress)
}

func (bs *BrokerStatus) MarkIngressFailed(err error) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionIngress, "failed", "%v", err)
}

func (bs *BrokerStatus) MarkTriggerChannelReady() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionTriggerChannel)
}

func (bs *BrokerStatus) MarkTriggerChannelFailed(err error) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionTriggerChannel, "failed", "%v", err)
}

func (bs *BrokerStatus) MarkIngressChannelReady() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionIngressChannel)
}

func (bs *BrokerStatus) MarkIngressChannelFailed(err error) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionIngressChannel, "failed", "%v", err)
}

func (bs *BrokerStatus) MarkIngressSubscriptionReady() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionIngressSubscription)
}

func (bs *BrokerStatus) MarkIngressSubscriptionFailed(err error) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionIngressSubscription, "failed", "%v", err)
}

func (bs *BrokerStatus) MarkFilterReady() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionFilter)
}

func (bs *BrokerStatus) MarkFilterFailed(err error) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionFilter, "failed", "%v", err)
}

// SetAddress makes this Broker addressable by setting the hostname. It also
// sets the BrokerConditionAddressable to true.
func (bs *BrokerStatus) SetAddress(hostname string) {
	bs.Address.Hostname = hostname
	if hostname != "" {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionAddressable)
	} else {
		brokerCondSet.Manage(bs).MarkFalse(BrokerConditionAddressable, "emptyHostname", "hostname is the empty string")
	}
}
