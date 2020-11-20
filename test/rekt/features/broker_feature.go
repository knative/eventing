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
	"fmt"

	"knative.dev/reconciler-test/pkg/feature"

	"knative.dev/eventing/test/rekt/resources/broker"
	brokercfg "knative.dev/eventing/test/rekt/resources/broker"
)

// BrokerGoesReady returns a feature that will create a Broker of the given
// name and class, and confirm it becomes ready with an address.
func BrokerGoesReady(name, class string) *feature.Feature {
	cfg := []brokercfg.CfgFn(nil)

	f := new(feature.Feature)

	// Set the class of the broker.
	if class != "" {
		cfg = append(cfg, brokercfg.WithBrokerClass(class))
	}

	f.Setup(fmt.Sprintf("install broker %q", name), broker.Install(name, cfg...))

	f.Stable("broker").
		Must("be ready", broker.IsReady(name, interval, timeout)).
		Must("be addressable", broker.IsAddressable(name, interval, timeout))

	return f
}
