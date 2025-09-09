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

package authz

import (
	"context"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/rekt/resources/eventpolicy"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/reconciler-test/pkg/environment"

	"knative.dev/eventing/test/rekt/features/featureflags"

	"github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventassert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
)

// AddressableAuthZConformance returns a feature set to test all Authorization features for an addressable.
func AddressableAuthZConformance(gvr schema.GroupVersionResource, kind, name string) *feature.FeatureSet {
	fs := feature.FeatureSet{
		Name: fmt.Sprintf("%s handles authorization features correctly", kind),
		Features: []*feature.Feature{
			addressableRespectsEventPolicyFilters(gvr, kind, name, cloudevents.EncodingBinary),
			addressableRespectsEventPolicyFilters(gvr, kind, name, cloudevents.EncodingStructured),
		},
	}

	fs.Features = append(fs.Features, AddressableAuthZConformanceRequestHandling(gvr, kind, name).Features...)

	return &fs
}

// AddressableAuthZConformanceRequestHandling returns a FeatureSet to test the basic authorization features.
// This basic feature set contains to allow authorized and reject unauthorized requests. In addition it also
// tests, that the addressable becomes unready in case of a NotReady assigned EventPolicy.
func AddressableAuthZConformanceRequestHandling(gvr schema.GroupVersionResource, kind, name string) *feature.FeatureSet {
	fs := feature.FeatureSet{
		Name: fmt.Sprintf("%s handles authorization in requests correctly", kind),
		Features: []*feature.Feature{
			addressableAllowsAuthorizedRequest(gvr, kind, name, cloudevents.EncodingBinary),
			addressableAllowsAuthorizedRequest(gvr, kind, name, cloudevents.EncodingStructured),
			addressableRejectsUnauthorizedRequest(gvr, kind, name, cloudevents.EncodingBinary),
			addressableRejectsUnauthorizedRequest(gvr, kind, name, cloudevents.EncodingStructured),
			addressableBecomesUnreadyOnUnreadyEventPolicy(gvr, kind, name),
		},
	}
	return &fs
}

func addressableAllowsAuthorizedRequest(gvr schema.GroupVersionResource, kind, name string, inputEventEncoding cloudevents.Encoding) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("%s accepts authorized request with %s encoding for input event", kind, inputEventEncoding))

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	source := feature.MakeRandomK8sName("source")
	eventPolicy := feature.MakeRandomK8sName("eventpolicy")
	sourceSubject := feature.MakeRandomK8sName("source-oidc-identity")

	event := test.FullEvent()

	// Install event policy
	f.Setup("Install the EventPolicy", func(ctx context.Context, t feature.T) {
		namespace := environment.FromContext(ctx).Namespace()
		eventpolicy.Install(
			eventPolicy,
			eventpolicy.WithToRef(
				gvr.GroupVersion().WithKind(kind),
				name),
			eventpolicy.WithFromSubject(fmt.Sprintf("system:serviceaccount:%s:%s", namespace, sourceSubject)),
		)(ctx, t)
	})
	f.Setup(fmt.Sprintf("EventPolicy for %s %s is ready", kind, name), k8s.IsReady(eventpolicy.GVR(), eventPolicy))

	// Install source
	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResourceTLS(gvr, name, nil),
		eventshub.InputEventWithEncoding(event, inputEventEncoding),
		eventshub.OIDCSubject(sourceSubject),
	))

	f.Alpha(kind).
		Must("event sent", eventassert.OnStore(source).MatchSentEvent(test.HasId(event.ID())).Exact(1)).
		Must("get 202 on response", eventassert.OnStore(source).Match(eventassert.MatchStatusCode(202)).AtLeast(1))

	return f
}

func addressableRejectsUnauthorizedRequest(gvr schema.GroupVersionResource, kind, name string, inputEventEncoding cloudevents.Encoding) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("%s rejects unauthorized request with %s encoding for input event", kind, inputEventEncoding))

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	source := feature.MakeRandomK8sName("source")
	eventPolicy := feature.MakeRandomK8sName("eventpolicy")

	event := test.FullEvent()

	// Install event policy
	f.Setup("Install the EventPolicy with from subject that does not match", eventpolicy.Install(
		eventPolicy,
		eventpolicy.WithToRef(
			gvr.GroupVersion().WithKind(kind),
			name),
		eventpolicy.WithFromSubject("system:serviceaccount:default:unknown-identity"),
	))
	f.Setup(fmt.Sprintf("EventPolicy for %s %s is ready", kind, name), k8s.IsReady(eventpolicy.GVR(), eventPolicy))

	// Install source
	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResourceTLS(gvr, name, nil),
		eventshub.InputEventWithEncoding(event, inputEventEncoding),
		eventshub.InitialSenderDelay(10*time.Second),
	))

	f.Alpha(kind).
		Must("event sent", eventassert.OnStore(source).MatchSentEvent(test.HasId(event.ID())).Exact(1)).
		Must("get 403 on response", eventassert.OnStore(source).Match(eventassert.MatchStatusCode(403)).AtLeast(1))

	return f
}

func addressableRespectsEventPolicyFilters(gvr schema.GroupVersionResource, kind, name string, inputEventEncoding cloudevents.Encoding) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("%s only admits events that pass the event policy filter with %s encoding for input event", kind, inputEventEncoding))

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	eventPolicy := feature.MakeRandomK8sName("eventpolicy")
	source1 := feature.MakeRandomK8sName("source")
	sourceSubject1 := feature.MakeRandomK8sName("source-oidc-identity")
	source2 := feature.MakeRandomK8sName("source")
	sourceSubject2 := feature.MakeRandomK8sName("source-oidc-identity")

	event1 := test.FullEvent()
	event1.SetType("valid.event.type")
	event1.SetID("1")
	event2 := test.FullEvent()
	event2.SetType("invalid.event.type")
	event2.SetID("2")

	// Install event policy
	f.Setup("Install the EventPolicy", func(ctx context.Context, t feature.T) {
		namespace := environment.FromContext(ctx).Namespace()
		eventpolicy.Install(
			eventPolicy,
			eventpolicy.WithToRef(
				gvr.GroupVersion().WithKind(kind),
				name),
			eventpolicy.WithFromSubject(fmt.Sprintf("system:serviceaccount:%s:%s", namespace, sourceSubject1)),
			eventpolicy.WithFromSubject(fmt.Sprintf("system:serviceaccount:%s:%s", namespace, sourceSubject2)),
			eventpolicy.WithFilters([]eventingv1.SubscriptionsAPIFilter{
				{
					Prefix: map[string]string{
						"type": "valid",
					},
				},
			}),
		)(ctx, t)
	})
	f.Setup(fmt.Sprintf("EventPolicy for %s %s is ready", kind, name), k8s.IsReady(eventpolicy.GVR(), eventPolicy))

	// Install source
	f.Requirement("install source 1", eventshub.Install(
		source1,
		eventshub.StartSenderToResourceTLS(gvr, name, nil),
		eventshub.InputEventWithEncoding(event1, inputEventEncoding),
		eventshub.OIDCSubject(sourceSubject1),
	))

	f.Requirement("install source 2", eventshub.Install(
		source2,
		eventshub.StartSenderToResourceTLS(gvr, name, nil),
		eventshub.InputEventWithEncoding(event2, inputEventEncoding),
		eventshub.OIDCSubject(sourceSubject2),
	))

	f.Alpha(kind).
		Must("valid event sent", eventassert.OnStore(source1).MatchSentEvent(test.HasId(event1.ID())).Exact(1)).
		Must("get 202 on response", eventassert.OnStore(source1).Match(eventassert.MatchStatusCode(202)).AtLeast(1))

	f.Alpha(kind).
		Must("invalid event sent", eventassert.OnStore(source2).MatchSentEvent(test.HasId(event2.ID())).Exact(1)).
		Must("get 403 on response", eventassert.OnStore(source2).Match(eventassert.MatchStatusCode(403)).AtLeast(1))

	return f
}

func addressableBecomesUnreadyOnUnreadyEventPolicy(gvr schema.GroupVersionResource, kind, name string) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("%s becomes NotReady when EventPolicy is NotReady", kind))

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	eventPolicy := feature.MakeRandomK8sName("eventpolicy")

	f.Setup(fmt.Sprintf("%s is ready initially", kind), k8s.IsReady(gvr, name))

	// Install event policy
	f.Requirement("Install the EventPolicy", eventpolicy.Install(
		eventPolicy,
		eventpolicy.WithToRef(
			gvr.GroupVersion().WithKind(kind),
			name),
		eventpolicy.WithFromRef(pingsource.Gvr().GroupVersion().WithKind("PingSource"), "doesnt-exist", "doesnt-exist"),
	))
	f.Requirement(fmt.Sprintf("EventPolicy for %s %s is NotReady", kind, name), k8s.IsNotReady(eventpolicy.GVR(), eventPolicy))

	f.Alpha(kind).Must("become NotReady with NotReady EventPolicy ", k8s.IsNotReady(gvr, name))

	return f
}
