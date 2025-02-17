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

	"knative.dev/eventing/pkg/client/certmanager/injection/informers/acme/v1/challenge"
	v1certificate "knative.dev/eventing/pkg/client/certmanager/injection/informers/certmanager/v1/certificate"

	"knative.dev/eventing/pkg/client/certmanager/injection/informers/certmanager/v1/certificaterequest"
	"knative.dev/eventing/pkg/client/certmanager/injection/informers/certmanager/v1/clusterissuer"
	"knative.dev/eventing/pkg/client/certmanager/injection/informers/certmanager/v1/issuer"
	"knative.dev/pkg/injection"

	"knative.dev/pkg/injection/sharedmain"

	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/signals"

	eventingfilteredfactory "knative.dev/eventing/pkg/client/injection/informers/factory/filtered"

	"knative.dev/eventing/pkg/apis/sinks"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/reconciler/eventpolicy"
	"knative.dev/eventing/pkg/reconciler/eventtransform"
	"knative.dev/eventing/pkg/reconciler/jobsink"

	"knative.dev/eventing/pkg/reconciler/apiserversource"
	"knative.dev/eventing/pkg/reconciler/channel"
	"knative.dev/eventing/pkg/reconciler/containersource"
	"knative.dev/eventing/pkg/reconciler/eventtype"
	integrationsink "knative.dev/eventing/pkg/reconciler/integration/sink"
	integrationsource "knative.dev/eventing/pkg/reconciler/integration/source"
	"knative.dev/eventing/pkg/reconciler/parallel"
	"knative.dev/eventing/pkg/reconciler/pingsource"
	"knative.dev/eventing/pkg/reconciler/sequence"
	sourcecrd "knative.dev/eventing/pkg/reconciler/source/crd"
	"knative.dev/eventing/pkg/reconciler/subscription"
	sugarnamespace "knative.dev/eventing/pkg/reconciler/sugar/namespace"
	sugartrigger "knative.dev/eventing/pkg/reconciler/sugar/trigger"
)

func main() {
	ctx := signals.NewContext()

	ctx = filteredFactory.WithSelectors(ctx,
		auth.OIDCLabelSelector,
		eventingtls.TrustBundleLabelSelector,
		sinks.JobSinkJobsLabelSelector,
		eventtransform.JsonataResourcesSelector,
		"app.kubernetes.io/name",
	)

	ctx = eventingfilteredfactory.WithSelectors(ctx,
		eventtransform.JsonataResourcesSelector,
	)

	for _, inf := range []injection.InformerInjector{challenge.WithInformer, v1certificate.WithInformer, certificaterequest.WithInformer, clusterissuer.WithInformer, issuer.WithInformer} {
		injection.Default.RegisterInformer(inf)
	}

	sharedmain.MainWithContext(ctx, "controller",
		// Messaging
		channel.NewController,
		subscription.NewController,

		// Eventing
		eventtype.NewController,
		eventpolicy.NewController,

		// Flows
		parallel.NewController,
		sequence.NewController,

		// Sources
		apiserversource.NewController,
		pingsource.NewController,
		containersource.NewController,
		integrationsource.NewController,

		// Sources CRD
		sourcecrd.NewController,

		// Sinks
		jobsink.NewController,
		integrationsink.NewController,

		// Sugar
		sugarnamespace.NewController,
		sugartrigger.NewController,

		// Transform
		eventtransform.NewController,
	)
}
