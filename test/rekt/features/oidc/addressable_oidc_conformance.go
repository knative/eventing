/*
Copyright 2024 The Knative Authors

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
	"fmt"

	"knative.dev/eventing/test/rekt/features/featureflags"

	"github.com/cloudevents/sdk-go/v2/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventassert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
)

func AddressableOIDCConformance(gvr schema.GroupVersionResource, kind, name, namespace string) *feature.FeatureSet {
	fs := feature.FeatureSet{
		Name:     fmt.Sprintf("%s handles requests with OIDC tokens correctly", kind),
		Features: AddressableOIDCTokenConformance(gvr, kind, name).Features,
	}

	fs.Features = append(fs.Features,
		AddressableHasAudiencePopulated(gvr, kind, name, namespace),
	)

	return &fs
}

func AddressableOIDCTokenConformance(gvr schema.GroupVersionResource, kind, name string) *feature.FeatureSet {
	fs := feature.FeatureSet{
		Name: fmt.Sprintf("%s handles requests with OIDC tokens correctly", kind),
		Features: []*feature.Feature{
			addressableRejectInvalidAudience(gvr, kind, name),
			addressableRejectCorruptedSignature(gvr, kind, name),
			addressableRejectExpiredToken(gvr, kind, name),
			addressableAllowsValidRequest(gvr, kind, name),
		},
	}

	return &fs
}

func AddressableHasAudiencePopulated(gvr schema.GroupVersionResource, kind, name, namespace string) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("%s populates its .status.address.audience correctly", kind))

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())

	f.Requirement(fmt.Sprintf("%s is ready", kind), k8s.IsReady(gvr, name))
	f.Requirement(fmt.Sprintf("%s is addressable", kind), k8s.IsAddressable(gvr, name))

	expectedAudience := auth.GetAudience(gvr.GroupVersion().WithKind(kind), metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	})

	f.Alpha(kind).Must("have audience set", addressable.ValidateAddress(gvr, name, addressable.AssertAddressWithAudience(expectedAudience)))

	return f
}

func addressableRejectInvalidAudience(gvr schema.GroupVersionResource, kind, name string) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("%s reject event for wrong OIDC audience", kind))

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	source := feature.MakeRandomK8sName("source")

	event := test.FullEvent()

	f.Requirement(fmt.Sprintf("%s is ready", kind), k8s.IsReady(gvr, name))
	f.Requirement(fmt.Sprintf("%s is addressable", kind), k8s.IsAddressable(gvr, name))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResourceTLS(gvr, name, nil),
		eventshub.OIDCInvalidAudience(),
		eventshub.InputEvent(event),
	))

	f.Alpha(kind).
		Must("event sent", eventassert.OnStore(source).MatchSentEvent(test.HasId(event.ID())).Exact(1)).
		Must("get 401 on response", eventassert.OnStore(source).Match(eventassert.MatchStatusCode(401)).Exact(1))

	return f
}

func addressableRejectExpiredToken(gvr schema.GroupVersionResource, kind, name string) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("%s reject event with expired OIDC token", kind))

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	source := feature.MakeRandomK8sName("source")

	event := test.FullEvent()

	f.Requirement(fmt.Sprintf("%s is ready", kind), k8s.IsReady(gvr, name))
	f.Requirement(fmt.Sprintf("%s is addressable", kind), k8s.IsAddressable(gvr, name))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResourceTLS(gvr, name, nil),
		eventshub.OIDCExpiredToken(),
		eventshub.InputEvent(event),
	))

	f.Alpha(kind).
		Must("event sent", eventassert.OnStore(source).MatchSentEvent(test.HasId(event.ID())).Exact(1)).
		Must("get 401 on response", eventassert.OnStore(source).Match(eventassert.MatchStatusCode(401)).Exact(1))

	return f
}

func addressableRejectCorruptedSignature(gvr schema.GroupVersionResource, kind, name string) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("%s reject event with corrupted OIDC token signature", kind))

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	source := feature.MakeRandomK8sName("source")

	event := test.FullEvent()

	f.Requirement(fmt.Sprintf("%s is ready", kind), k8s.IsReady(gvr, name))
	f.Requirement(fmt.Sprintf("%s is addressable", kind), k8s.IsAddressable(gvr, name))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResourceTLS(gvr, name, nil),
		eventshub.OIDCCorruptedSignature(),
		eventshub.InputEvent(event),
	))

	f.Alpha(kind).
		Must("event sent", eventassert.OnStore(source).MatchSentEvent(test.HasId(event.ID())).Exact(1)).
		Must("get 401 on response", eventassert.OnStore(source).Match(eventassert.MatchStatusCode(401)).Exact(1))

	return f
}

func addressableAllowsValidRequest(gvr schema.GroupVersionResource, kind, name string) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("%s handles event with valid OIDC token", kind))

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	source := feature.MakeRandomK8sName("source")

	event := test.FullEvent()

	f.Requirement(fmt.Sprintf("%s is ready", kind), k8s.IsReady(gvr, name))
	f.Requirement(fmt.Sprintf("%s is addressable", kind), k8s.IsAddressable(gvr, name))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResourceTLS(gvr, name, nil),
		eventshub.InputEvent(event),
	))

	f.Alpha(kind).
		Must("event sent", eventassert.OnStore(source).MatchSentEvent(test.HasId(event.ID())).Exact(1)).
		Must("get 202 on response", eventassert.OnStore(source).Match(eventassert.MatchStatusCode(202)).Exact(1))

	return f
}
