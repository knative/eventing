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

package resources

// This file contains functions that construct Messaging resources.

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	pkgTest "knative.dev/pkg/test"
)

// SequenceOption enables further configuration of a Sequence.
type SequenceOption func(*messagingv1alpha1.Sequence)

// WithReplyForSequence returns an option that adds a Reply for the given Sequence.
func WithReplyForSequence(name string, typemeta *metav1.TypeMeta) SequenceOption {
	return func(seq *messagingv1alpha1.Sequence) {
		if name != "" {
			seq.Spec.Reply = pkgTest.CoreV1ObjectReference(typemeta.Kind, typemeta.APIVersion, name)
		}
	}
}

// Sequence returns a Sequence resource.
func Sequence(
	name string,
	steps []eventingv1alpha1.SubscriberSpec,
	channelTemplate *eventingduckv1alpha1.ChannelTemplateSpec,
	options ...SequenceOption,
) *messagingv1alpha1.Sequence {
	sequence := &messagingv1alpha1.Sequence{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: messagingv1alpha1.SequenceSpec{
			Steps:           steps,
			ChannelTemplate: channelTemplate,
		},
	}
	for _, option := range options {
		option(sequence)
	}
	return sequence
}
