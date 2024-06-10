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

package trigger

import (
	"context"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func CrossNamespaceEventLinks(brokerEnvCtx context.Context) *feature.Feature {
	f := feature.NewFeature()

	f.Prerequisite("Cross Namespace Event Links is enabled", featureflags.CrossEventLinksEnabled())

	sourceName := feature.MakeRandomK8sName("source")
	subscriberName := feature.MakeRandomK8sName("subscriber")

	ev := cetest.FullEvent()

	triggerName := feature.MakeRandomK8sName("trigger")
	brokerName := feature.MakeRandomK8sName("broker")
	brokerNamespace := environment.FromContext(brokerEnvCtx).Namespace()

	triggerCfg := []manifest.CfgFn{
		trigger.WithSubscriber(service.AsKReference(subscriberName), ""),
		trigger.WithBrokerRefName(brokerName),
		trigger.WithBrokerRefNamespace(brokerNamespace),
	}

	f.Setup("install broker", broker.Install(brokerName, broker.WithNamespace(brokerNamespace)))
	f.Setup("install trigger", trigger.Install(triggerName, triggerCfg...))

	f.Setup("install subscriber", eventshub.Install(subscriberName, eventshub.StartReceiver))
	f.Setup("install event source", eventshub.Install(sourceName, eventshub.StartSenderToNamespacedResource(broker.GVR(), brokerName, brokerNamespace), eventshub.InputEvent(ev)))

	f.Assert("event is received by subscriber", assert.OnStore(subscriberName).MatchEvent(cetest.HasId(ev.ID())).Exact(1))

	return f
}
