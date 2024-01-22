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
	"context"

	"knative.dev/eventing/test/rekt/features/featureflags"

	"github.com/cloudevents/sdk-go/v2/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/reconciler/sequence/resources"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/channel_template"
	"knative.dev/eventing/test/rekt/resources/sequence"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
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

func SequenceSendsEventWithOIDC() *feature.FeatureSet {
	return &feature.FeatureSet{
		Name: "Sequence send events with OIDC support",
		Features: []*feature.Feature{
			SequenceSendsEventWithOIDCTokenToSteps(),
			SequenceSendsEventWithOIDCTokenToReply(),
		},
	}
}

func SequenceSendsEventWithOIDCTokenToSteps() *feature.Feature {
	f := feature.NewFeatureNamed("Sequence supports OIDC in internal flow between steps")

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	channelTemplate := channel_template.ChannelTemplate{
		TypeMeta: channel_impl.TypeMeta(),
		Spec:     map[string]interface{}{},
	}

	sequenceName := feature.MakeRandomK8sName("sequence")
	step1Name := feature.MakeRandomK8sName("step1")
	step2Name := feature.MakeRandomK8sName("step2")
	sourceName := feature.MakeRandomK8sName("source")

	step1Audience := "step1-aud"
	step2Audience := "step2-aud"

	step1Append := "-step1"
	step2Append := "-step2"

	f.Setup("install step 1", eventshub.Install(step1Name,
		eventshub.ReplyWithAppendedData(step1Append),
		eventshub.OIDCReceiverAudience(step1Audience),
		eventshub.StartReceiverTLS))
	f.Setup("install step 2", eventshub.Install(step2Name,
		eventshub.ReplyWithAppendedData(step2Append),
		eventshub.OIDCReceiverAudience(step2Audience),
		eventshub.StartReceiverTLS))

	f.Setup("Install Sequence", func(ctx context.Context, t feature.T) {
		cfg := []manifest.CfgFn{
			sequence.WithChannelTemplate(channelTemplate),
			sequence.WithStepFromDestination(&duckv1.Destination{
				Ref:      service.AsKReference(step1Name),
				Audience: &step1Audience,
				CACerts:  eventshub.GetCaCerts(ctx),
			}),
			sequence.WithStepFromDestination(&duckv1.Destination{
				Ref:      service.AsKReference(step2Name),
				Audience: &step2Audience,
				CACerts:  eventshub.GetCaCerts(ctx),
			}),
		}

		sequence.Install(sequenceName, cfg...)(ctx, t)
	})

	f.Setup("Sequence goes ready", sequence.IsReady(sequenceName))

	event := test.FullEvent()
	event.SetData("text/plain", "hello")
	f.Requirement("install source", eventshub.Install(sourceName,
		eventshub.StartSenderToResourceTLS(sequence.GVR(), sequenceName, nil),
		eventshub.InputEvent(event)))

	expectedMsg := string(event.Data())
	expectedMsg += step1Append
	f.Alpha("Sequence with steps having an OIDC audience").
		Must("Delivers events correctly to steps",
			assert.OnStore(step2Name).MatchEvent(
				test.HasData([]byte(expectedMsg)),
			).AtLeast(1))

	return f
}

func SequenceSendsEventWithOIDCTokenToReply() *feature.Feature {
	f := feature.NewFeatureNamed("Sequence supports OIDC for reply")

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	channelTemplate := channel_template.ChannelTemplate{
		TypeMeta: channel_impl.TypeMeta(),
		Spec:     map[string]interface{}{},
	}

	sequenceName := feature.MakeRandomK8sName("sequence")
	step1Name := feature.MakeRandomK8sName("step1")
	step2Name := feature.MakeRandomK8sName("step2")
	replySinkName := feature.MakeRandomK8sName("reply-sink")
	sourceName := feature.MakeRandomK8sName("source")

	step1Audience := "step1-aud"
	step2Audience := "step2-aud"
	replySinkAudience := "reply-sink-aud"

	step1Append := "-step1"
	step2Append := "-step2"

	f.Setup("install step 1", eventshub.Install(step1Name,
		eventshub.ReplyWithAppendedData(step1Append),
		eventshub.OIDCReceiverAudience(step1Audience),
		eventshub.StartReceiverTLS))
	f.Setup("install step 2", eventshub.Install(step2Name,
		eventshub.ReplyWithAppendedData(step2Append),
		eventshub.OIDCReceiverAudience(step2Audience),
		eventshub.StartReceiverTLS))

	f.Setup("install sink", eventshub.Install(replySinkName,
		eventshub.OIDCReceiverAudience(replySinkAudience),
		eventshub.StartReceiverTLS))

	f.Setup("Install Sequence", func(ctx context.Context, t feature.T) {
		cfg := []manifest.CfgFn{
			sequence.WithChannelTemplate(channelTemplate),
			sequence.WithReplyFromDestination(&duckv1.Destination{
				Ref:      service.AsKReference(replySinkName),
				Audience: &replySinkAudience,
				CACerts:  eventshub.GetCaCerts(ctx),
			}),
			sequence.WithStepFromDestination(&duckv1.Destination{
				Ref:      service.AsKReference(step1Name),
				Audience: &step1Audience,
				CACerts:  eventshub.GetCaCerts(ctx),
			}),
			sequence.WithStepFromDestination(&duckv1.Destination{
				Ref:      service.AsKReference(step2Name),
				Audience: &step2Audience,
				CACerts:  eventshub.GetCaCerts(ctx),
			}),
		}

		sequence.Install(sequenceName, cfg...)(ctx, t)
	})
	f.Setup("Sequence goes ready", sequence.IsReady(sequenceName))

	event := test.FullEvent()
	event.SetData("text/plain", "hello")
	f.Requirement("install source", eventshub.Install(sourceName,
		eventshub.StartSenderToResourceTLS(sequence.GVR(), sequenceName, nil),
		eventshub.InputEvent(event)))

	expectedMsg := string(event.Data())
	expectedMsg += step1Append
	expectedMsg += step2Append
	f.Alpha("Sequence with steps having an OIDC audience").
		Must("Delivers events correctly to reply",
			assert.OnStore(replySinkName).MatchEvent(
				test.HasData([]byte(expectedMsg)),
			).AtLeast(1))

	return f
}
