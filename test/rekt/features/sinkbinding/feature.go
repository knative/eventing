/*
Copyright 2022 The Knative Authors

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

package sinkbinding

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/deployment"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/eventing/test/rekt/resources/job"
	"knative.dev/eventing/test/rekt/resources/sinkbinding"
)

const (
	heartbeatsImage = "ko://knative.dev/eventing/test/test_images/heartbeats"
)

func SinkBindingV1Deployment(ctx context.Context) *feature.Feature {
	sbinding := feature.MakeRandomK8sName("sinkbinding")
	sink := feature.MakeRandomK8sName("sink")
	subject := feature.MakeRandomK8sName("subject")
	extensionSecret := string(uuid.NewUUID())

	f := feature.NewFeatureNamed("SinkBinding V1 Deployment test")

	env := environment.FromContext(ctx)
	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install a deployment", deployment.Install(subject, heartbeatsImage,
		deployment.WithEnvs(map[string]string{
			"POD_NAME":      "heartbeats",
			"POD_NAMESPACE": env.Namespace(),
		})))

	extensions := map[string]string{
		"sinkbinding": extensionSecret,
	}

	cfg := []manifest.CfgFn{
		sinkbinding.WithExtensions(extensions),
	}

	f.Requirement("install SinkBinding", sinkbinding.Install(sbinding, service.AsDestinationRef(sink), deployment.AsTrackerReference(subject), cfg...))
	f.Requirement("SinkBinding goes ready", sinkbinding.IsReady(sbinding))

	f.Stable("Create a deployment as sinkbinding's subject").
		Must("delivers events",
			eventasssert.OnStore(sink).MatchEvent(
				test.HasExtension("sinkbinding", extensionSecret),
			).AtLeast(1))

	return f
}

func SinkBindingV1Job() *feature.Feature {
	sbinding := feature.MakeRandomK8sName("sinkbinding")
	sink := feature.MakeRandomK8sName("sink")
	subject := feature.MakeRandomK8sName("subject")
	extensionSecret := string(uuid.NewUUID())

	f := feature.NewFeatureNamed("SinkBinding goes ready")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install a Job", job.Install(subject))

	extensions := map[string]string{
		"sinkbinding": extensionSecret,
	}

	cfg := []manifest.CfgFn{
		sinkbinding.WithExtensions(extensions),
	}

	f.Setup("install SinkBinding", sinkbinding.Install(sbinding, service.AsDestinationRef(sink), job.AsTrackerReference(subject), cfg...))
	f.Setup("SinkBinding goes ready", sinkbinding.IsReady(sbinding))

	f.Stable("Create a job as sinkbinding's subject").
		Must("delivers events",
			eventasssert.OnStore(sink).MatchEvent(
				test.HasExtension("sinkbinding", extensionSecret),
			).AtLeast(1))

	return f
}
