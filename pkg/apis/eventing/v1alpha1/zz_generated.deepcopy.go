// +build !ignore_autogenerated

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
<<<<<<< HEAD
<<<<<<< HEAD
func (in *Callable) DeepCopyInto(out *Callable) {
	*out = *in
	if in.Target != nil {
		in, out := &in.Target, &out.Target
		if *in == nil {
			*out = nil
		} else {
			*out = new(v1.ObjectReference)
			**out = **in
		}
	}
	if in.TargetURI != nil {
		in, out := &in.TargetURI, &out.TargetURI
		if *in == nil {
			*out = nil
		} else {
			*out = new(string)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Callable.
func (in *Callable) DeepCopy() *Callable {
	if in == nil {
		return nil
	}
	out := new(Callable)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProvisionedStatus) DeepCopyInto(out *ProvisionedStatus) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProvisionedStatus.
func (in *ProvisionedStatus) DeepCopy() *ProvisionedStatus {
	if in == nil {
		return nil
	}
	out := new(ProvisionedStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
=======
>>>>>>> trim more 0.1 object model down.
func (in *Provisioner) DeepCopyInto(out *Provisioner) {
=======
func (in *ClusterProvisioner) DeepCopyInto(out *ClusterProvisioner) {
>>>>>>> Switch to cluster provisoner.
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterProvisioner.
func (in *ClusterProvisioner) DeepCopy() *ClusterProvisioner {
	if in == nil {
		return nil
	}
	out := new(ClusterProvisioner)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterProvisioner) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterProvisionerConditionStatus) DeepCopyInto(out *ClusterProvisionerConditionStatus) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterProvisionerConditionStatus.
func (in *ClusterProvisionerConditionStatus) DeepCopy() *ClusterProvisionerConditionStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterProvisionerConditionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterProvisionerList) DeepCopyInto(out *ClusterProvisionerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterProvisioner, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterProvisionerList.
func (in *ClusterProvisionerList) DeepCopy() *ClusterProvisionerList {
	if in == nil {
		return nil
	}
	out := new(ClusterProvisionerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterProvisionerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterProvisionerSpec) DeepCopyInto(out *ClusterProvisionerSpec) {
	*out = *in
	out.Type = in.Type
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterProvisionerSpec.
func (in *ClusterProvisionerSpec) DeepCopy() *ClusterProvisionerSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterProvisionerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterProvisionerStatus) DeepCopyInto(out *ClusterProvisionerStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]ClusterProvisionerConditionStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterProvisionerStatus.
func (in *ClusterProvisionerStatus) DeepCopy() *ClusterProvisionerStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterProvisionerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResultStrategy) DeepCopyInto(out *ResultStrategy) {
	*out = *in
	if in.Target != nil {
		in, out := &in.Target, &out.Target
		if *in == nil {
			*out = nil
		} else {
			*out = new(v1.ObjectReference)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResultStrategy.
func (in *ResultStrategy) DeepCopy() *ResultStrategy {
	if in == nil {
		return nil
	}
	out := new(ResultStrategy)
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
func (in *SubscriptionCondition) DeepCopyInto(out *SubscriptionCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubscriptionCondition.
func (in *SubscriptionCondition) DeepCopy() *SubscriptionCondition {
	if in == nil {
		return nil
	}
	out := new(SubscriptionCondition)
	in.DeepCopyInto(out)
	return out
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
	out.From = in.From
	if in.Call != nil {
		in, out := &in.Call, &out.Call
		if *in == nil {
			*out = nil
		} else {
			*out = new(Callable)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Result != nil {
		in, out := &in.Result, &out.Result
		if *in == nil {
			*out = nil
		} else {
			*out = new(ResultStrategy)
			(*in).DeepCopyInto(*out)
		}
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
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]SubscriptionCondition, len(*in))
		copy(*out, *in)
	}
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
