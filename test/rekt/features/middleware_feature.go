/*
Copyright 2020 The Knative Authors

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

package features

import (
	"context"
	"fmt"
	"testing"

	"knative.dev/eventing/test/rekt/resources/svc"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"

	. "github.com/cloudevents/sdk-go/v2/test"

	"knative.dev/eventing/test/rekt/resources/broker"
)

// BrokerAsMiddleware tests to see if the Broker acts as middleware.
// LoadGenerator --> in [Broker] out --> Recorder
func BrokerAsMiddleware(brokerName string) *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")
	event := FullEvent()

	f := new(feature.Feature)

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install source", eventshub.Install(source, eventshub.StartSender(sink), eventshub.InputEvent(event)))

	// Point the Trigger subscriber to the sink svc.
	cfg := []trigger.CfgFn{trigger.WithSubscriber(svc.AsRef(sink), "")}

	// Install the trigger
	f.Setup(fmt.Sprintf("install trigger %q", via), trigger.Install(via, brokerName, cfg...))

	// Wait for broker to be Ready.
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName, interval, timeout))

	f.Stable("broker as middleware").
		Must("lossless event passing",
			func(ctx context.Context, t *testing.T) {
				eventshub.StoreFromContext(ctx, sink).AssertExact(1, eventshub.MatchEvent(HasId(event.ID())))
			})

	return f
}
