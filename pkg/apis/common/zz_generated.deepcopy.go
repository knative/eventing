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

package common

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSCommon) DeepCopyInto(out *AWSCommon) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSCommon.
func (in *AWSCommon) DeepCopy() *AWSCommon {
	if in == nil {
		return nil
	}
	out := new(AWSCommon)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSDDBStreams) DeepCopyInto(out *AWSDDBStreams) {
	*out = *in
	out.AWSCommon = in.AWSCommon
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSDDBStreams.
func (in *AWSDDBStreams) DeepCopy() *AWSDDBStreams {
	if in == nil {
		return nil
	}
	out := new(AWSDDBStreams)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSS3) DeepCopyInto(out *AWSS3) {
	*out = *in
	out.AWSCommon = in.AWSCommon
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSS3.
func (in *AWSS3) DeepCopy() *AWSS3 {
	if in == nil {
		return nil
	}
	out := new(AWSS3)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSSQS) DeepCopyInto(out *AWSSQS) {
	*out = *in
	out.AWSCommon = in.AWSCommon
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSSQS.
func (in *AWSSQS) DeepCopy() *AWSSQS {
	if in == nil {
		return nil
	}
	out := new(AWSSQS)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Auth) DeepCopyInto(out *Auth) {
	*out = *in
	if in.Secret != nil {
		in, out := &in.Secret, &out.Secret
		*out = new(Secret)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Auth.
func (in *Auth) DeepCopy() *Auth {
	if in == nil {
		return nil
	}
	out := new(Auth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Secret) DeepCopyInto(out *Secret) {
	*out = *in
	if in.Ref != nil {
		in, out := &in.Ref, &out.Ref
		*out = new(SecretReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Secret.
func (in *Secret) DeepCopy() *Secret {
	if in == nil {
		return nil
	}
	out := new(Secret)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretReference) DeepCopyInto(out *SecretReference) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretReference.
func (in *SecretReference) DeepCopy() *SecretReference {
	if in == nil {
		return nil
	}
	out := new(SecretReference)
	in.DeepCopyInto(out)
	return out
}
