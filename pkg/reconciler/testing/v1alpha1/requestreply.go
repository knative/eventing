/*
Copyright 2025 The Knative Authors

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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	eventing "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// RequestReplyOption enables further configuration of a RequestReply.
type RequestReplyOption func(rr *eventing.RequestReply)

// NewRequestReply() creates a RequestReply with RequestReplyOptions.
func NewRequestReply(name, namespace string, o ...RequestReplyOption) *eventing.RequestReply {
	rr := &eventing.RequestReply{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(rr)
	}
	rr.SetDefaults(context.Background())
	return rr
}

func WithRequestReplyBroker(b duckv1.KReference) RequestReplyOption {
	return func(rr *eventing.RequestReply) {
		rr.Spec.BrokerRef = b
	}
}

func WithInitRequestReplyConditions(rr *eventing.RequestReply) {
	rr.Status.InitializeConditions()
}

func WithRequestReplyReady(rr *eventing.RequestReply) {
	url, _ := apis.ParseURL("request-reply.knative-testing.svc.cluster.local")
	rr.Status.MarkTriggersReady()
	rr.Status.MarkBrokerReady()
	rr.Status.MarkEventPoliciesTrueWithReason("NotImplemented", "Event policies not implemented for RequestReply yet")
	address := &duckv1.Addressable{
		Name: ptr.To("http"),
		URL:  url,
	}
	address.URL.Path = "/test-namespace/test-requestReply"
	rr.Status.SetAddress(address)
}

func WithRequestReplyAddress(address *duckv1.Addressable) RequestReplyOption {
	return func(rr *eventing.RequestReply) {
		rr.Status.SetAddress(address)
	}
}

func WithRequestReplyTriggersReady(rr *eventing.RequestReply) {
	rr.Status.MarkTriggersReady()
}

func WithRequestReplyBrokerReady(rr *eventing.RequestReply) {
	rr.Status.MarkBrokerReady()
}

func WithRequestReplyEventPoliciesReady(rr *eventing.RequestReply) {
	rr.Status.MarkEventPoliciesTrueWithReason("NotImplemented", "Event policies not implemented for RequestReply yet")
}

func WithRequestReplyReplicas(desired, ready int32) RequestReplyOption {
	return func(rr *eventing.RequestReply) {
		rr.Status.DesiredReplicas = ptr.To(desired)
		rr.Status.ReadyReplicas = ptr.To(ready)
	}
}

func WithRequestReplyBrokerAddressAnnotation(address string) RequestReplyOption {
	return func(rr *eventing.RequestReply) {
		if rr.Status.Annotations == nil {
			rr.Status.Annotations = make(map[string]string)
		}
		rr.Status.Annotations[eventing.RequestReplyBrokerAddressStatusAnnotationKey] = address
	}
}
