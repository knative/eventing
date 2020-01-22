/*
Copyright 2019 The Knative Authors

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

package main

import (
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"knative.dev/pkg/injection/sharedmain"

	"knative.dev/eventing/pkg/reconciler/broker"
	"knative.dev/eventing/pkg/reconciler/channel"
	"knative.dev/eventing/pkg/reconciler/eventtype"
	flowsparallel "knative.dev/eventing/pkg/reconciler/flowsparallel"
	flowssequence "knative.dev/eventing/pkg/reconciler/flowssequence"
	"knative.dev/eventing/pkg/reconciler/namespace"
	"knative.dev/eventing/pkg/reconciler/subscription"
	"knative.dev/eventing/pkg/reconciler/trigger"
)

func main() {
	sharedmain.Main("controller",
		subscription.NewController,
		namespace.NewController,
		channel.NewController,
		trigger.NewController,
		broker.NewController,
		eventtype.NewController,
		flowsparallel.NewController,
		flowssequence.NewController,
	)
}
