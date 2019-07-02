// +build !ignore_autogenerated

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	duckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Broker) DeepCopyInto(out *Broker) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Broker.
func (in *Broker) DeepCopy() *Broker {
	if in == nil {
		return nil
	}
	out := new(Broker)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Broker) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BrokerList) DeepCopyInto(out *BrokerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Broker, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BrokerList.
func (in *BrokerList) DeepCopy() *BrokerList {
	if in == nil {
		return nil
	}
	out := new(BrokerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BrokerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BrokerSpec) DeepCopyInto(out *BrokerSpec) {
	*out = *in
	if in.DeprecatedChannelTemplate != nil {
		in, out := &in.DeprecatedChannelTemplate, &out.DeprecatedChannelTemplate
		*out = new(ChannelSpec)
		(*in).DeepCopyInto(*out)
	}
	in.ChannelTemplate.DeepCopyInto(&out.ChannelTemplate)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BrokerSpec.
func (in *BrokerSpec) DeepCopy() *BrokerSpec {
	if in == nil {
		return nil
	}
	out := new(BrokerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BrokerStatus) DeepCopyInto(out *BrokerStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
	in.Address.DeepCopyInto(&out.Address)
	if in.TriggerChannel != nil {
		in, out := &in.TriggerChannel, &out.TriggerChannel
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.IngressChannel != nil {
		in, out := &in.IngressChannel, &out.IngressChannel
		*out = new(v1.ObjectReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BrokerStatus.
func (in *BrokerStatus) DeepCopy() *BrokerStatus {
	if in == nil {
		return nil
	}
	out := new(BrokerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Channel) DeepCopyInto(out *Channel) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Channel.
func (in *Channel) DeepCopy() *Channel {
	if in == nil {
		return nil
	}
	out := new(Channel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Channel) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChannelList) DeepCopyInto(out *ChannelList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Channel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChannelList.
func (in *ChannelList) DeepCopy() *ChannelList {
	if in == nil {
		return nil
	}
	out := new(ChannelList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ChannelList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChannelSpec) DeepCopyInto(out *ChannelSpec) {
	*out = *in
	if in.Provisioner != nil {
		in, out := &in.Provisioner, &out.Provisioner
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.Arguments != nil {
		in, out := &in.Arguments, &out.Arguments
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
	if in.Subscribable != nil {
		in, out := &in.Subscribable, &out.Subscribable
		*out = new(duckv1alpha1.Subscribable)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChannelSpec.
func (in *ChannelSpec) DeepCopy() *ChannelSpec {
	if in == nil {
		return nil
	}
	out := new(ChannelSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChannelStatus) DeepCopyInto(out *ChannelStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
	in.Address.DeepCopyInto(&out.Address)
	if in.Internal != nil {
		in, out := &in.Internal, &out.Internal
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
	in.SubscribableTypeStatus.DeepCopyInto(&out.SubscribableTypeStatus)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChannelStatus.
func (in *ChannelStatus) DeepCopy() *ChannelStatus {
	if in == nil {
		return nil
	}
	out := new(ChannelStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChannelTemplateSpec) DeepCopyInto(out *ChannelTemplateSpec) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChannelTemplateSpec.
func (in *ChannelTemplateSpec) DeepCopy() *ChannelTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(ChannelTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ChannelTemplateSpec) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChannelTemplateSpecInternal) DeepCopyInto(out *ChannelTemplateSpecInternal) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChannelTemplateSpecInternal.
func (in *ChannelTemplateSpecInternal) DeepCopy() *ChannelTemplateSpecInternal {
	if in == nil {
		return nil
	}
	out := new(ChannelTemplateSpecInternal)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ChannelTemplateSpecInternal) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterChannelProvisioner) DeepCopyInto(out *ClusterChannelProvisioner) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterChannelProvisioner.
func (in *ClusterChannelProvisioner) DeepCopy() *ClusterChannelProvisioner {
	if in == nil {
		return nil
	}
	out := new(ClusterChannelProvisioner)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterChannelProvisioner) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterChannelProvisionerList) DeepCopyInto(out *ClusterChannelProvisionerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterChannelProvisioner, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterChannelProvisionerList.
func (in *ClusterChannelProvisionerList) DeepCopy() *ClusterChannelProvisionerList {
	if in == nil {
		return nil
	}
	out := new(ClusterChannelProvisionerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterChannelProvisionerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterChannelProvisionerSpec) DeepCopyInto(out *ClusterChannelProvisionerSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterChannelProvisionerSpec.
func (in *ClusterChannelProvisionerSpec) DeepCopy() *ClusterChannelProvisionerSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterChannelProvisionerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterChannelProvisionerStatus) DeepCopyInto(out *ClusterChannelProvisionerStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterChannelProvisionerStatus.
func (in *ClusterChannelProvisionerStatus) DeepCopy() *ClusterChannelProvisionerStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterChannelProvisionerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventType) DeepCopyInto(out *EventType) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventType.
func (in *EventType) DeepCopy() *EventType {
	if in == nil {
		return nil
	}
	out := new(EventType)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EventType) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventTypeList) DeepCopyInto(out *EventTypeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]EventType, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventTypeList.
func (in *EventTypeList) DeepCopy() *EventTypeList {
	if in == nil {
		return nil
	}
	out := new(EventTypeList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EventTypeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventTypeSpec) DeepCopyInto(out *EventTypeSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventTypeSpec.
func (in *EventTypeSpec) DeepCopy() *EventTypeSpec {
	if in == nil {
		return nil
	}
	out := new(EventTypeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventTypeStatus) DeepCopyInto(out *EventTypeStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventTypeStatus.
func (in *EventTypeStatus) DeepCopy() *EventTypeStatus {
	if in == nil {
		return nil
	}
	out := new(EventTypeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReplyStrategy) DeepCopyInto(out *ReplyStrategy) {
	*out = *in
	if in.Channel != nil {
		in, out := &in.Channel, &out.Channel
		*out = new(v1.ObjectReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReplyStrategy.
func (in *ReplyStrategy) DeepCopy() *ReplyStrategy {
	if in == nil {
		return nil
	}
	out := new(ReplyStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubscriberSpec) DeepCopyInto(out *SubscriberSpec) {
	*out = *in
	if in.Ref != nil {
		in, out := &in.Ref, &out.Ref
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.DeprecatedDNSName != nil {
		in, out := &in.DeprecatedDNSName, &out.DeprecatedDNSName
		*out = new(string)
		**out = **in
	}
	if in.URI != nil {
		in, out := &in.URI, &out.URI
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubscriberSpec.
func (in *SubscriberSpec) DeepCopy() *SubscriberSpec {
	if in == nil {
		return nil
	}
	out := new(SubscriberSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Subscription) DeepCopyInto(out *Subscription) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Subscription.
func (in *Subscription) DeepCopy() *Subscription {
	if in == nil {
		return nil
	}
	out := new(Subscription)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Subscription) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubscriptionList) DeepCopyInto(out *SubscriptionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Subscription, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubscriptionList.
func (in *SubscriptionList) DeepCopy() *SubscriptionList {
	if in == nil {
		return nil
	}
	out := new(SubscriptionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SubscriptionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubscriptionSpec) DeepCopyInto(out *SubscriptionSpec) {
	*out = *in
	out.Channel = in.Channel
	if in.Subscriber != nil {
		in, out := &in.Subscriber, &out.Subscriber
		*out = new(SubscriberSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Reply != nil {
		in, out := &in.Reply, &out.Reply
		*out = new(ReplyStrategy)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubscriptionSpec.
func (in *SubscriptionSpec) DeepCopy() *SubscriptionSpec {
	if in == nil {
		return nil
	}
	out := new(SubscriptionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubscriptionStatus) DeepCopyInto(out *SubscriptionStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
	out.PhysicalSubscription = in.PhysicalSubscription
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubscriptionStatus.
func (in *SubscriptionStatus) DeepCopy() *SubscriptionStatus {
	if in == nil {
		return nil
	}
	out := new(SubscriptionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubscriptionStatusPhysicalSubscription) DeepCopyInto(out *SubscriptionStatusPhysicalSubscription) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubscriptionStatusPhysicalSubscription.
func (in *SubscriptionStatusPhysicalSubscription) DeepCopy() *SubscriptionStatusPhysicalSubscription {
	if in == nil {
		return nil
	}
	out := new(SubscriptionStatusPhysicalSubscription)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Trigger) DeepCopyInto(out *Trigger) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Trigger.
func (in *Trigger) DeepCopy() *Trigger {
	if in == nil {
		return nil
	}
	out := new(Trigger)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Trigger) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TriggerFilter) DeepCopyInto(out *TriggerFilter) {
	*out = *in
	if in.SourceAndType != nil {
		in, out := &in.SourceAndType, &out.SourceAndType
		*out = new(TriggerFilterSourceAndType)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TriggerFilter.
func (in *TriggerFilter) DeepCopy() *TriggerFilter {
	if in == nil {
		return nil
	}
	out := new(TriggerFilter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TriggerFilterSourceAndType) DeepCopyInto(out *TriggerFilterSourceAndType) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TriggerFilterSourceAndType.
func (in *TriggerFilterSourceAndType) DeepCopy() *TriggerFilterSourceAndType {
	if in == nil {
		return nil
	}
	out := new(TriggerFilterSourceAndType)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TriggerList) DeepCopyInto(out *TriggerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Trigger, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TriggerList.
func (in *TriggerList) DeepCopy() *TriggerList {
	if in == nil {
		return nil
	}
	out := new(TriggerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TriggerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TriggerSpec) DeepCopyInto(out *TriggerSpec) {
	*out = *in
	if in.Filter != nil {
		in, out := &in.Filter, &out.Filter
		*out = new(TriggerFilter)
		(*in).DeepCopyInto(*out)
	}
	if in.Subscriber != nil {
		in, out := &in.Subscriber, &out.Subscriber
		*out = new(SubscriberSpec)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TriggerSpec.
func (in *TriggerSpec) DeepCopy() *TriggerSpec {
	if in == nil {
		return nil
	}
	out := new(TriggerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TriggerStatus) DeepCopyInto(out *TriggerStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
	in.Address.DeepCopyInto(&out.Address)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TriggerStatus.
func (in *TriggerStatus) DeepCopy() *TriggerStatus {
	if in == nil {
		return nil
	}
	out := new(TriggerStatus)
	in.DeepCopyInto(out)
	return out
}
