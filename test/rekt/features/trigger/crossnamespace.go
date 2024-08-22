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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
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

	brokerRef := &duckv1.KReference{
		APIVersion: "eventing.knative.dev/v1",
		Kind:       "Broker",
		Name:       brokerName,
		Namespace:  brokerNamespace,
	}

	triggerCfg := []manifest.CfgFn{
		trigger.WithSubscriber(service.AsKReference(subscriberName), ""),
		trigger.WithBrokerRef(brokerRef),
	}

	f.Setup("install broker", broker.Install(brokerName, append(broker.WithEnvConfig(), broker.WithNamespace(brokerNamespace))...))
	f.Setup("install trigger", trigger.Install(triggerName, triggerCfg...))

	f.Setup("install subscriber", eventshub.Install(subscriberName, eventshub.StartReceiver))

	// <resource>.IsReady uses the environment in the context to find the resource, hence we can only check the trigger
	// However, the trigger being ready implies the broker is ready, so we are okay
	f.Setup("trigger is ready", trigger.IsReady(triggerName))
	f.Requirement("install event source", eventshub.Install(sourceName, eventshub.StartSenderToNamespacedResource(broker.GVR(), brokerName, brokerNamespace), eventshub.InputEvent(ev)))

	f.Assert("event is received by subscriber", assert.OnStore(subscriberName).MatchEvent(cetest.HasId(ev.ID())).Exact(1))

	f.Teardown("delete trigger", func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		eventingclient.Get(ctx).EventingV1().Triggers(env.Namespace()).Delete(ctx, triggerName, metav1.DeleteOptions{})
	})

	return f
}
