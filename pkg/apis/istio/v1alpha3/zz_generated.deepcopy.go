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

package v1alpha3

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CorsPolicy) DeepCopyInto(out *CorsPolicy) {
	*out = *in
	if in.AllowOrigin != nil {
		in, out := &in.AllowOrigin, &out.AllowOrigin
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AllowMethods != nil {
		in, out := &in.AllowMethods, &out.AllowMethods
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AllowHeaders != nil {
		in, out := &in.AllowHeaders, &out.AllowHeaders
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ExposeHeaders != nil {
		in, out := &in.ExposeHeaders, &out.ExposeHeaders
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CorsPolicy.
func (in *CorsPolicy) DeepCopy() *CorsPolicy {
	if in == nil {
		return nil
	}
	out := new(CorsPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Destination) DeepCopyInto(out *Destination) {
	*out = *in
	out.Port = in.Port
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Destination.
func (in *Destination) DeepCopy() *Destination {
	if in == nil {
		return nil
	}
	out := new(Destination)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DestinationWeight) DeepCopyInto(out *DestinationWeight) {
	*out = *in
	out.Destination = in.Destination
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DestinationWeight.
func (in *DestinationWeight) DeepCopy() *DestinationWeight {
	if in == nil {
		return nil
	}
	out := new(DestinationWeight)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Gateway) DeepCopyInto(out *Gateway) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Gateway.
func (in *Gateway) DeepCopy() *Gateway {
	if in == nil {
		return nil
	}
	out := new(Gateway)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Gateway) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GatewayList) DeepCopyInto(out *GatewayList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Gateway, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GatewayList.
func (in *GatewayList) DeepCopy() *GatewayList {
	if in == nil {
		return nil
	}
	out := new(GatewayList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GatewayList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GatewaySpec) DeepCopyInto(out *GatewaySpec) {
	*out = *in
	if in.Servers != nil {
		in, out := &in.Servers, &out.Servers
		*out = make([]Server, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GatewaySpec.
func (in *GatewaySpec) DeepCopy() *GatewaySpec {
	if in == nil {
		return nil
	}
	out := new(GatewaySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPFaultInjection) DeepCopyInto(out *HTTPFaultInjection) {
	*out = *in
	if in.Delay != nil {
		in, out := &in.Delay, &out.Delay
		if *in == nil {
			*out = nil
		} else {
			*out = new(InjectDelay)
			**out = **in
		}
	}
	if in.Abort != nil {
		in, out := &in.Abort, &out.Abort
		if *in == nil {
			*out = nil
		} else {
			*out = new(InjectAbort)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPFaultInjection.
func (in *HTTPFaultInjection) DeepCopy() *HTTPFaultInjection {
	if in == nil {
		return nil
	}
	out := new(HTTPFaultInjection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPMatchRequest) DeepCopyInto(out *HTTPMatchRequest) {
	*out = *in
	if in.Uri != nil {
		in, out := &in.Uri, &out.Uri
		if *in == nil {
			*out = nil
		} else {
			*out = new(StringMatch)
			**out = **in
		}
	}
	if in.Scheme != nil {
		in, out := &in.Scheme, &out.Scheme
		if *in == nil {
			*out = nil
		} else {
			*out = new(StringMatch)
			**out = **in
		}
	}
	if in.Method != nil {
		in, out := &in.Method, &out.Method
		if *in == nil {
			*out = nil
		} else {
			*out = new(StringMatch)
			**out = **in
		}
	}
	if in.Authority != nil {
		in, out := &in.Authority, &out.Authority
		if *in == nil {
			*out = nil
		} else {
			*out = new(StringMatch)
			**out = **in
		}
	}
	if in.Headers != nil {
		in, out := &in.Headers, &out.Headers
		*out = make(map[string]StringMatch, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPMatchRequest.
func (in *HTTPMatchRequest) DeepCopy() *HTTPMatchRequest {
	if in == nil {
		return nil
	}
	out := new(HTTPMatchRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPRedirect) DeepCopyInto(out *HTTPRedirect) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRedirect.
func (in *HTTPRedirect) DeepCopy() *HTTPRedirect {
	if in == nil {
		return nil
	}
	out := new(HTTPRedirect)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPRetry) DeepCopyInto(out *HTTPRetry) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRetry.
func (in *HTTPRetry) DeepCopy() *HTTPRetry {
	if in == nil {
		return nil
	}
	out := new(HTTPRetry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPRewrite) DeepCopyInto(out *HTTPRewrite) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRewrite.
func (in *HTTPRewrite) DeepCopy() *HTTPRewrite {
	if in == nil {
		return nil
	}
	out := new(HTTPRewrite)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPRoute) DeepCopyInto(out *HTTPRoute) {
	*out = *in
	if in.Match != nil {
		in, out := &in.Match, &out.Match
		*out = make([]HTTPMatchRequest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Route != nil {
		in, out := &in.Route, &out.Route
		*out = make([]DestinationWeight, len(*in))
		copy(*out, *in)
	}
	if in.Redirect != nil {
		in, out := &in.Redirect, &out.Redirect
		if *in == nil {
			*out = nil
		} else {
			*out = new(HTTPRedirect)
			**out = **in
		}
	}
	if in.Rewrite != nil {
		in, out := &in.Rewrite, &out.Rewrite
		if *in == nil {
			*out = nil
		} else {
			*out = new(HTTPRewrite)
			**out = **in
		}
	}
	if in.Retries != nil {
		in, out := &in.Retries, &out.Retries
		if *in == nil {
			*out = nil
		} else {
			*out = new(HTTPRetry)
			**out = **in
		}
	}
	if in.Fault != nil {
		in, out := &in.Fault, &out.Fault
		if *in == nil {
			*out = nil
		} else {
			*out = new(HTTPFaultInjection)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Mirror != nil {
		in, out := &in.Mirror, &out.Mirror
		if *in == nil {
			*out = nil
		} else {
			*out = new(Destination)
			**out = **in
		}
	}
	if in.AppendHeaders != nil {
		in, out := &in.AppendHeaders, &out.AppendHeaders
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.RemoveResponseHeaders != nil {
		in, out := &in.RemoveResponseHeaders, &out.RemoveResponseHeaders
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRoute.
func (in *HTTPRoute) DeepCopy() *HTTPRoute {
	if in == nil {
		return nil
	}
	out := new(HTTPRoute)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InjectAbort) DeepCopyInto(out *InjectAbort) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InjectAbort.
func (in *InjectAbort) DeepCopy() *InjectAbort {
	if in == nil {
		return nil
	}
	out := new(InjectAbort)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InjectDelay) DeepCopyInto(out *InjectDelay) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InjectDelay.
func (in *InjectDelay) DeepCopy() *InjectDelay {
	if in == nil {
		return nil
	}
	out := new(InjectDelay)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *L4MatchAttributes) DeepCopyInto(out *L4MatchAttributes) {
	*out = *in
	if in.SourceLabel != nil {
		in, out := &in.SourceLabel, &out.SourceLabel
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Gateways != nil {
		in, out := &in.Gateways, &out.Gateways
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new L4MatchAttributes.
func (in *L4MatchAttributes) DeepCopy() *L4MatchAttributes {
	if in == nil {
		return nil
	}
	out := new(L4MatchAttributes)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Port) DeepCopyInto(out *Port) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Port.
func (in *Port) DeepCopy() *Port {
	if in == nil {
		return nil
	}
	out := new(Port)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PortSelector) DeepCopyInto(out *PortSelector) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PortSelector.
func (in *PortSelector) DeepCopy() *PortSelector {
	if in == nil {
		return nil
	}
	out := new(PortSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Server) DeepCopyInto(out *Server) {
	*out = *in
	out.Port = in.Port
	if in.Hosts != nil {
		in, out := &in.Hosts, &out.Hosts
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.TLS != nil {
		in, out := &in.TLS, &out.TLS
		if *in == nil {
			*out = nil
		} else {
			*out = new(TLSOptions)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Server.
func (in *Server) DeepCopy() *Server {
	if in == nil {
		return nil
	}
	out := new(Server)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StringMatch) DeepCopyInto(out *StringMatch) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StringMatch.
func (in *StringMatch) DeepCopy() *StringMatch {
	if in == nil {
		return nil
	}
	out := new(StringMatch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TCPRoute) DeepCopyInto(out *TCPRoute) {
	*out = *in
	if in.Match != nil {
		in, out := &in.Match, &out.Match
		*out = make([]L4MatchAttributes, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.Route = in.Route
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TCPRoute.
func (in *TCPRoute) DeepCopy() *TCPRoute {
	if in == nil {
		return nil
	}
	out := new(TCPRoute)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TLSOptions) DeepCopyInto(out *TLSOptions) {
	*out = *in
	if in.SubjectAltNames != nil {
		in, out := &in.SubjectAltNames, &out.SubjectAltNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TLSOptions.
func (in *TLSOptions) DeepCopy() *TLSOptions {
	if in == nil {
		return nil
	}
	out := new(TLSOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualService) DeepCopyInto(out *VirtualService) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualService.
func (in *VirtualService) DeepCopy() *VirtualService {
	if in == nil {
		return nil
	}
	out := new(VirtualService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VirtualService) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualServiceList) DeepCopyInto(out *VirtualServiceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VirtualService, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualServiceList.
func (in *VirtualServiceList) DeepCopy() *VirtualServiceList {
	if in == nil {
		return nil
	}
	out := new(VirtualServiceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VirtualServiceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualServiceSpec) DeepCopyInto(out *VirtualServiceSpec) {
	*out = *in
	if in.Hosts != nil {
		in, out := &in.Hosts, &out.Hosts
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Gateways != nil {
		in, out := &in.Gateways, &out.Gateways
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Http != nil {
		in, out := &in.Http, &out.Http
		*out = make([]HTTPRoute, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Tcp != nil {
		in, out := &in.Tcp, &out.Tcp
		*out = make([]TCPRoute, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualServiceSpec.
func (in *VirtualServiceSpec) DeepCopy() *VirtualServiceSpec {
	if in == nil {
		return nil
	}
	out := new(VirtualServiceSpec)
	in.DeepCopyInto(out)
	return out
}
