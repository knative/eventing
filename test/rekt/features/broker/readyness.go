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

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/svc"
	"knative.dev/eventing/test/rekt/resources/trigger"
)

// TriggerGoesReady returns a feature that tests after the creation of a
// Trigger, it becomes ready. This feature assumes the Broker already exists.
func TriggerGoesReady(name, brokerName string) *feature.Feature {
	cfg := []manifest.CfgFn(nil)

	f := new(feature.Feature)

	// The test needs a subscriber.
	sub := feature.MakeRandomK8sName("sub")
	f.Setup("install a service", svc.Install(sub, "app", "rekt"))

	// Point the Trigger subscriber to the service installed in setup.
	cfg = append(cfg, trigger.WithSubscriber(svc.AsRef(name), ""))

	// Install the trigger
	f.Setup(fmt.Sprintf("install trigger %q", name), trigger.Install(name, brokerName, cfg...))

	// Wait for a ready broker.
	f.Requirement("broker is ready", broker.IsReady(brokerName))

	f.Stable("trigger").
		Must("be ready", trigger.IsReady(name))

	return f
}

// GoesReady returns a feature that will create a Broker of the given
// name and class, and confirm it becomes ready with an address.
func GoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := new(feature.Feature)

	f.Setup(fmt.Sprintf("install broker %q", name), broker.Install(name, cfg...))

	f.Stable("broker").
		Must("be ready", broker.IsReady(name)).
		Must("be addressable", broker.IsAddressable(name))

	return f
}
