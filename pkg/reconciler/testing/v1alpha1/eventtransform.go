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

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventing "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	sources "knative.dev/eventing/pkg/apis/sources/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

// EventTransformOption enables further configuration of a EventTransform.
type EventTransformOption func(transform *eventing.EventTransform)

// NewEventTransform creates an EventTransform with EventTransformOptions.
func NewEventTransform(name, namespace string, o ...EventTransformOption) *eventing.EventTransform {
	t := &eventing.EventTransform{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(t)
	}
	t.SetDefaults(context.Background())
	return t
}

func WithEventTransformJsonataExpression() EventTransformOption {
	return func(transform *eventing.EventTransform) {
		const expression = `{"id": id}`
		if transform.Spec.Jsonata == nil {
			transform.Spec.Jsonata = &eventing.JsonataEventTransformationSpec{Expression: expression}
			return
		}
		transform.Spec.Jsonata.Expression = expression
	}
}

func WithEventTransformJsonataReplyExpression() EventTransformOption {
	return func(transform *eventing.EventTransform) {
		const expression = `{"reply": reply}`
		if transform.Spec.Reply == nil {
			transform.Spec.Reply = &eventing.ReplySpec{
				EventTransformations: eventing.EventTransformations{
					Jsonata: &eventing.JsonataEventTransformationSpec{},
				},
			}
		}
		transform.Spec.Reply.Jsonata.Expression = expression
	}
}

func WithEventTransformJsonataReplyDiscard() EventTransformOption {
	return func(transform *eventing.EventTransform) {
		if transform.Spec.Reply == nil {
			transform.Spec.Reply = &eventing.ReplySpec{}
		}
		transform.Spec.Reply.Discard = ptr.Bool(true)
	}
}

func WithEventTransformSink(d duckv1.Destination) EventTransformOption {
	return func(transform *eventing.EventTransform) {
		transform.Spec.Sink = &d
	}
}

func WithJsonataEventTransformInitializeStatus() EventTransformOption {
	return func(transform *eventing.EventTransform) {
		transform.Status.InitializeConditions()
		if transform.Status.JsonataTransformationStatus == nil {
			transform.Status.JsonataTransformationStatus = &eventing.JsonataEventTransformationStatus{}
		}
		if transform.Spec.Sink == nil {
			transform.Status.PropagateJsonataSinkBindingUnset()
		}
	}
}

func WithJsonataDeploymentStatus(status appsv1.DeploymentStatus) EventTransformOption {
	return func(transform *eventing.EventTransform) {
		transform.Status.PropagateJsonataDeploymentStatus(status)
	}
}

func WithJsonataCertificateStatus(status cmv1.CertificateStatus) EventTransformOption {
	return func(transform *eventing.EventTransform) {
		transform.Status.PropagateJsonataCertificateStatus(status)
	}
}

func WithJsonataSinkBindingStatus(status sources.SinkBindingStatus) EventTransformOption {
	return func(transform *eventing.EventTransform) {
		transform.Status.PropagateJsonataSinkBindingStatus(status)
	}
}

func WithJsonataWaitingForServiceEndpoints() EventTransformOption {
	return func(transform *eventing.EventTransform) {
		transform.Status.MarkWaitingForServiceEndpoints()
	}
}

func WithEventTransformAddresses(addr ...duckv1.Addressable) EventTransformOption {
	return func(transform *eventing.EventTransform) {
		transform.Status.SetAddresses(addr...)
	}
}
