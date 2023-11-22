/*
Copyright 2023 The Knative Authors

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

package oidc

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/reconciler/sequence/resources"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/sequence"
	"knative.dev/reconciler-test/pkg/feature"
)

func SequenceHasAudienceOfInputChannel(sequenceName, sequenceNamespace string, channelGVR schema.GroupVersionResource, channelKind string) *feature.Feature {
	f := feature.NewFeatureNamed("Sequence has audience of input channel")

	f.Setup("Sequence goes ready", sequence.IsReady(sequenceName))

	expectedAudience := auth.GetAudience(channelGVR.GroupVersion().WithKind(channelKind), metav1.ObjectMeta{
		Name:      resources.SequenceChannelName(sequenceName, 0),
		Namespace: sequenceNamespace,
	})

	f.Alpha("Sequence").Must("has audience set", sequence.ValidateAddress(sequenceName, addressable.AssertAddressWithAudience(expectedAudience)))

	return f
}
