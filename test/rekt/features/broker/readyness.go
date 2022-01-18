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

package broker

import (
	"fmt"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"

	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/trigger"
)

// TriggerGoesReady returns a feature that tests after the creation of a
// Trigger, it becomes ready. This feature assumes the Broker already exists.
func TriggerGoesReady(name, brokerName string, cfg ...manifest.CfgFn) *feature.Feature {
	f := new(feature.Feature)

	// The test needs a subscriber.
	sub := feature.MakeRandomK8sName("sub")
	f.Setup("install a service", svc.Install(sub, "app", "rekt"))
	// Append user-provided cfg to the end, in case they are providing their own subscriber.
	cfg = append([]manifest.CfgFn{trigger.WithSubscriber(svc.AsKReference(sub), "")}, cfg...)

	// Install the trigger
	f.Setup(fmt.Sprintf("install trigger %q", name), trigger.Install(name, brokerName, cfg...))

	// Wait for a ready broker.
	f.Requirement("Broker is ready", broker.IsReady(brokerName))
	f.Requirement("Trigger is ready", trigger.IsReady(name))

	f.Stable("trigger")

	return f
}

// GoesReady returns a feature that will create a Broker of the given
// name and class, and confirm it becomes ready with an address.
func GoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := new(feature.Feature)

	f.Setup(fmt.Sprintf("install broker %q", name), broker.Install(name, cfg...))

	f.Requirement("Broker is ready", broker.IsReady(name))

	f.Stable("broker").
		Must("be addressable", broker.IsAddressable(name))

	return f
}

// GoesReadyWithProbeReceiver returns a feature that will create a Broker of the given
// name and class, with an event receiver as its DLS
// and confirm it becomes ready with an address.
func GoesReadyWithProbeReceiver(name, sinkName string, prober *eventshub.EventProber, cfg ...manifest.CfgFn) *feature.Feature {
	f := new(feature.Feature)

	f.Setup("install probe", prober.ReceiverInstall(sinkName))

	brokerConfig := append(cfg, delivery.WithDeadLetterSink(prober.AsKReference(sinkName), ""))
	f.Setup(fmt.Sprintf("install broker %q", name), broker.Install(name, brokerConfig...))

	f.Requirement("Broker is ready", broker.IsReady(name))

	f.Stable("broker").
		Must("be addressable", broker.IsAddressable(name))

	return f
}
