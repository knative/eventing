//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2021 The Knative Authors

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
	runtime "k8s.io/apimachinery/pkg/runtime"
	apis "knative.dev/pkg/apis"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventTypeDefinition) DeepCopyInto(out *EventTypeDefinition) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventTypeDefinition.
func (in *EventTypeDefinition) DeepCopy() *EventTypeDefinition {
	if in == nil {
		return nil
	}
	out := new(EventTypeDefinition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EventTypeDefinition) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventTypeDefinitionAttribute) DeepCopyInto(out *EventTypeDefinitionAttribute) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventTypeDefinitionAttribute.
func (in *EventTypeDefinitionAttribute) DeepCopy() *EventTypeDefinitionAttribute {
	if in == nil {
		return nil
	}
	out := new(EventTypeDefinitionAttribute)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventTypeDefinitionList) DeepCopyInto(out *EventTypeDefinitionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]EventTypeDefinition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventTypeDefinitionList.
func (in *EventTypeDefinitionList) DeepCopy() *EventTypeDefinitionList {
	if in == nil {
		return nil
	}
	out := new(EventTypeDefinitionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EventTypeDefinitionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventTypeDefinitionMetadata) DeepCopyInto(out *EventTypeDefinitionMetadata) {
	*out = *in
	if in.Attributes != nil {
		in, out := &in.Attributes, &out.Attributes
		*out = make([]EventTypeDefinitionAttribute, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventTypeDefinitionMetadata.
func (in *EventTypeDefinitionMetadata) DeepCopy() *EventTypeDefinitionMetadata {
	if in == nil {
		return nil
	}
	out := new(EventTypeDefinitionMetadata)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventTypeDefinitionSpec) DeepCopyInto(out *EventTypeDefinitionSpec) {
	*out = *in
	if in.SchemaURL != nil {
		in, out := &in.SchemaURL, &out.SchemaURL
		*out = new(apis.URL)
		(*in).DeepCopyInto(*out)
	}
	in.Metadata.DeepCopyInto(&out.Metadata)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventTypeDefinitionSpec.
func (in *EventTypeDefinitionSpec) DeepCopy() *EventTypeDefinitionSpec {
	if in == nil {
		return nil
	}
	out := new(EventTypeDefinitionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EventTypeDefinitionStatus) DeepCopyInto(out *EventTypeDefinitionStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EventTypeDefinitionStatus.
func (in *EventTypeDefinitionStatus) DeepCopy() *EventTypeDefinitionStatus {
	if in == nil {
		return nil
	}
	out := new(EventTypeDefinitionStatus)
	in.DeepCopyInto(out)
	return out
}
