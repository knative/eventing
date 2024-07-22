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

	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

func GoesReadyInDifferentNamespace(name, namespace string, cfg ...manifest.CfgFn) *feature.Feature {
	f := new(feature.Feature)
	f.Prerequisite("Cross Namespace Event Links is enabled", featureflags.CrossEventLinksEnabled())

	// Add the namespace configuration
	namespaceCfg := broker.WithConfigNamespace(namespace)
	cfg = append(cfg, namespaceCfg)

	f.Setup(fmt.Sprintf("install broker %q in namespace %q", name, namespace), broker.Install(name, cfg...))
	f.Setup("Broker is ready", broker.IsReady(name))
	f.Setup("Broker is addressable", broker.IsAddressable(name))

	return f
}
