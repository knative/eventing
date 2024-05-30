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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	eventingv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1"
	"knative.dev/eventing/test/rekt/features/featureflags"
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
	brokerNamespace := environment.FromContext(brokerEnvCtx).Namespace() // To be passed to WithBrokerRef

	f.Setup("install subscriber", eventshub.Install(subscriberName, eventshub.StartReceiver))
	f.Setup("install event source", func(ctx context.Context, t feature.T) {
		config, err := rest.InClusterConfig()
		if err != nil {
			t.Fatal(err)
		}
		eventingClient, err := eventingv1.NewForConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		brokerURL, err := ResolveBrokerURL(ctx, brokerNamespace, brokerName, eventingClient)
		if err != nil {
			t.Fatal(err)
		}
		eventshub.Install(sourceName, eventshub.StartSenderURL(brokerURL), eventshub.InputEvent(ev))(ctx, t)
	})

	f.Setup("install trigger", trigger.Install(triggerName, trigger.WithBrokerName(brokerName), trigger.WithBrokerNamespace(brokerNamespace)))
	f.Setup("trigger is ready", trigger.IsReady(triggerName))

	f.Assert("event is received by subscriber", assert.OnStore(subscriberName).MatchEvent(cetest.HasId(ev.ID())).Exact(1))

	return f
}

func ResolveBrokerURL(ctx context.Context, brokerNamespace, brokerName string, client eventingv1.EventingV1Interface) (string, error) {
	var brokerURL string
	interval, timeout := environment.PollTimingsFromContext(ctx)
	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		broker, err := client.Brokers(brokerNamespace).Get(ctx, brokerName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if broker.Status.Address == nil || broker.Status.Address.URL == nil {
			return false, nil
		}
		brokerURL = broker.Status.Address.URL.String()
		return true, nil
	})
	return brokerURL, err
}
