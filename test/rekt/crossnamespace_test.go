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

package rekt

import (
	"testing"

	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	"knative.dev/eventing/test/rekt/features/broker"
	cn "knative.dev/eventing/test/rekt/features/crossnamespace"
	"knative.dev/eventing/test/rekt/features/featureflags"
)

func TestBrokerTriggerCrossNamespaceReference(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	// namespaces and names for the broker and trigger
	brokerNamespace := feature.MakeRandomK8sName("broker-namespace")
	triggerNamespace := feature.MakeRandomK8sName("trigger-namespace")
	brokerName := feature.MakeRandomK8sName("broker")
	triggerName := feature.MakeRandomK8sName("trigger")

	env.Prerequisite("Cross Namespace Event Links is enabled", featureflags.CrossEventLinksEnabled())
	// namespaces for the broker and trigger if they do not exist
	env.Prerequisite(ctx, t, env.CreateNamespaceIfNeeded(brokerNamespace))
	env.Prerequisite(ctx, t, env.CreateNamespaceIfNeeded(triggerNamespace))

	env.Test(ctx, t, broker.GoesReadyInDifferentNamespace(brokerName, brokerNamespace))
	env.Test(ctx, t, broker.TriggerGoesReady(triggerName, brokerName))
	env.Test(ctx, t, cn.TriggerSendsEventsToBroker(brokerNamespace, brokerName, triggerNamespace, triggerName))
}
