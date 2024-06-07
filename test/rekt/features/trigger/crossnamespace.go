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
	"log"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
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

	log.Printf("Setting up subscriber: %s", subscriberName)
	f.Setup("install subscriber", eventshub.Install(subscriberName, eventshub.StartReceiver))

	log.Printf("Setting up event source: %s", sourceName)
	f.Setup("install event source", eventshub.Install(sourceName, eventshub.StartSenderToNamespacedResource(broker.GVR(), brokerName, brokerNamespace), eventshub.InputEvent(ev)))

	log.Printf("Setting up broker: %s in namespace: %s", brokerName, brokerNamespace)
	log.Printf("Setting up trigger: %s for broker: %s", triggerName, brokerName)
	f.Setup("install trigger", trigger.Install(triggerName, trigger.WithBrokerName(brokerName), trigger.WithBrokerNamespace(brokerNamespace)))

	log.Printf("Waiting for trigger to be ready: %s", triggerName)
	f.Setup("trigger is ready", trigger.IsReady(triggerName))

	log.Printf("Asserting event is received by subscriber: %s", subscriberName)
	f.Assert("event is received by subscriber", assert.OnStore(subscriberName).MatchEvent(cetest.HasId(ev.ID())).Exact(1))

	return f
	return f
}
