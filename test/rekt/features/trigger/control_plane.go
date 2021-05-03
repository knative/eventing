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

package trigger

import (
	"context"

	"github.com/stretchr/testify/assert"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	triggerresources "knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/resources/svc"
)

func Defaulting() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Trigger Specification - Defaulting",
		Features: []feature.Feature{
			*Defaulting_Filter(),
			*Defaulting_SubscriberNamespace(),
		},
	}

	return fs
}

func Defaulting_Filter() *feature.Feature {
	f := feature.NewFeatureNamed("Broker")

	resourceName := feature.MakeRandomK8sName("trigger")

	withSubscriber := triggerresources.WithSubscriber(svc.AsKReference("sub"), "")

	f.Setup("Set Trigger name", SetTriggerName(resourceName))
	f.Setup("Create a Trigger with empty spec.filter",
		triggerresources.Install(resourceName, "broker", withSubscriber))

	f.Stable("Conformance").
		Must("Trigger MUST default spec.filter to empty filter",
			defaultFilterIsSetOnTrigger)
	return f
}

func Defaulting_SubscriberNamespace() *feature.Feature {
	f := feature.NewFeatureNamed("Broker")

	resourceName := feature.MakeRandomK8sName("trigger")

	withSubscriber := triggerresources.WithSubscriber(svc.AsKReference("sub"), "")

	f.Setup("Set Trigger name", SetTriggerName(resourceName))
	f.Setup("Create a Trigger with empty subscriber namespace",
		triggerresources.Install(resourceName, "broker", withSubscriber))

	f.Stable("Conformance").
		Must("Trigger subscriber namespace MUST be defaulted to Trigger namespace",
			triggerNamespaceIsSetOnSubscriber)
	return f
}

func defaultFilterIsSetOnTrigger(ctx context.Context, t feature.T) {
	trigger := GetTrigger(ctx, t)

	assert.Equal(t, &eventingv1.TriggerFilter{}, trigger.Spec.Filter)
}

func triggerNamespaceIsSetOnSubscriber(ctx context.Context, t feature.T) {
	trigger := GetTrigger(ctx, t)

	assert.Equal(t, trigger.Namespace, trigger.Spec.Subscriber.Ref.Namespace)
}
