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

package apiserversource

import (
	"context"
	"github.com/cloudevents/sdk-go/v2/test"
	rbacv1 "k8s.io/api/rbac/v1"
	"knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/features/source"
	"knative.dev/eventing/test/rekt/resources/account_role"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/pod"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func ApiserversourceSendEventWithJWT() *feature.Feature {
	src := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	audience := "my-sink-audience"
	sacmName := feature.MakeRandomK8sName("apiserversource")

	f := feature.NewFeatureNamed("ApiServerSource send events with OIDC authentication")

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("deploy receiver", eventshub.Install(sink,
		eventshub.StartReceiverTLS,
		eventshub.OIDCReceiverAudience(audience)))

	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForApiserversource(sacmName))

	cfg := []manifest.CfgFn{
		apiserversource.WithServiceAccountName(sacmName),
		apiserversource.WithEventMode(v1.ResourceMode),
		apiserversource.WithResources(v1.APIVersionKindSelector{
			APIVersion: "v1",
			Kind:       "Event",
		}),
	}

	f.Requirement("install ApiServerSource", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sink)
		d.Audience = &audience
		d.CACerts = eventshub.GetCaCerts(ctx)

		cfg = append(cfg, apiserversource.WithSink(d))
		apiserversource.Install(src, cfg...)(ctx, t)
	})
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(src))

	examplePodName := feature.MakeRandomK8sName("example")
	// create a pod so that ApiServerSource delivers an event to its sink
	// event body is similar to this:
	// {"kind":"Pod","namespace":"test-wmbcixlv","name":"example-axvlzbvc","apiVersion":"v1"}
	f.Requirement("install example pod", pod.Install(examplePodName, exampleImage))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on sink with ref",
			assert.OnStore(sink).
				Match(assert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).
				AtLeast(1),
		).Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(apiserversource.Gvr(), src)).
		Must("Set sinkCACerts to non empty CA certs", source.ExpectCACerts(apiserversource.Gvr(), src))

	return f
}

func setupAccountAndRoleForApiserversource(sacmName string) feature.StepFn {
	return account_role.Install(sacmName,
		account_role.WithRole(sacmName+"-clusterrole"),
		account_role.WithRules(rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch", "create"},
		}),
	)
}
