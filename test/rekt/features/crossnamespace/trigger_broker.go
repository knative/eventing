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

package crossnamespace

import (
	"context"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/require"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/resources/broker"
)

func TriggerSendsEventsToBroker(brokerNamespace, brokerName, triggerNamespace, triggerName string) *feature.Feature {
	f := feature.NewFeature()

	sourceName := feature.MakeRandomK8sName("source")
	subscriberName := feature.MakeRandomK8sName("subscriber")

	ev := cetest.FullEvent()

	f.Setup("install subscriber", eventshub.Install(subscriberName, eventshub.StartReceiver))
	f.Setup("install event source", eventshub.Install(sourceName, eventshub.StartSenderToResource(broker.GVR(), brokerName), eventshub.InputEvent(ev)))

	f.Requirement("trigger is ready", func(ctx context.Context, t feature.T) {
		client := eventingclient.Get(ctx)
		trig, err := client.EventingV1().Triggers(triggerNamespace).Get(ctx, triggerName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, trig.Status.IsReady(), true)
	})

	f.Assert("event is received by subscriber", assert.OnStore(subscriberName).MatchEvent(cetest.HasId(ev.ID())).Exact(1))

	return f
}
