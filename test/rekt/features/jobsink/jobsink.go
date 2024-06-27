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

package jobsink

import (
	"context"
	"encoding/json"
	"fmt"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"

	"knative.dev/eventing/pkg/apis/sinks"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/jobsink"
)

func Success() *feature.Feature {
	f := feature.NewFeature()

	sink := feature.MakeRandomK8sName("sink")
	jobSink := feature.MakeRandomK8sName("jobsink")
	source := feature.MakeRandomK8sName("source")

	sinkURL := &apis.URL{Scheme: "http", Host: sink}

	event := cetest.FullEvent()
	event.SetID(uuid.NewString())

	f.Setup("install forwarder sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install job sink", jobsink.Install(jobSink, jobsink.WithForwarderJob(sinkURL.String())))

	f.Setup("jobsink is addressable", jobsink.IsAddressable(jobSink))
	f.Setup("jobsink is ready", jobsink.IsAddressable(jobSink))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResource(jobsink.GVR(), jobSink),
		eventshub.InputEvent(event)))

	f.Assert("Job is created with the mounted event", assert.OnStore(sink).
		MatchReceivedEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("Source sent the event", assert.OnStore(source).
		Match(assert.MatchKind(eventshub.EventResponse)).
		Match(assert.MatchStatusCode(202)).
		AtLeast(1),
	)
	f.Assert("At least one Job is complete", AtLeastOneJobIsComplete(jobSink))

	return f
}

func SuccessTLS() *feature.Feature {
	f := feature.NewFeature()

	sink := feature.MakeRandomK8sName("sink")
	jobSink := feature.MakeRandomK8sName("jobsink")
	source := feature.MakeRandomK8sName("source")

	sinkURL := &apis.URL{Scheme: "http", Host: sink}

	event := cetest.FullEvent()
	event.SetID(uuid.NewString())

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install forwarder sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install job sink", jobsink.Install(jobSink, jobsink.WithForwarderJob(sinkURL.String())))

	f.Setup("jobsink is addressable", jobsink.IsAddressable(jobSink))
	f.Setup("jobsink is ready", jobsink.IsAddressable(jobSink))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResourceTLS(jobsink.GVR(), jobSink, nil),
		eventshub.InputEvent(event)))

	f.Assert("JobSink has https address", addressable.ValidateAddress(jobsink.GVR(), jobSink, addressable.AssertHTTPSAddress))
	f.Assert("Job is created with the mounted event", assert.OnStore(sink).
		MatchReceivedEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("Source sent the event", assert.OnStore(source).
		Match(assert.MatchKind(eventshub.EventResponse)).
		Match(assert.MatchStatusCode(202)).
		AtLeast(1),
	)
	f.Assert("At least one Job is complete", AtLeastOneJobIsComplete(jobSink))

	return f
}

func OIDC() *feature.Feature {
	f := feature.NewFeature()

	sink := feature.MakeRandomK8sName("sink")
	jobSink := feature.MakeRandomK8sName("jobsink")
	source := feature.MakeRandomK8sName("source")
	sourceNoAudience := feature.MakeRandomK8sName("source-no-audience")

	sinkURL := &apis.URL{Scheme: "http", Host: sink}

	event := cetest.FullEvent()
	event.SetID(uuid.NewString())

	eventNoAudience := cetest.FullEvent()
	eventNoAudience.SetID(uuid.NewString())

	f.Prerequisite("OIDC authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install forwarder sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install job sink", jobsink.Install(jobSink, jobsink.WithForwarderJob(sinkURL.String())))

	f.Setup("jobsink is addressable", jobsink.IsAddressable(jobSink))
	f.Setup("jobsink is ready", jobsink.IsAddressable(jobSink))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResource(jobsink.GVR(), jobSink),
		eventshub.InputEvent(event)))

	f.Requirement("install source no audience", func(ctx context.Context, t feature.T) {
		addr, err := jobsink.Address(ctx, jobSink)
		if err != nil {
			t.Error(err)
			return
		}
		eventshub.Install(sourceNoAudience,
			eventshub.StartSenderURLTLS(addr.URL.String(), addr.CACerts),
			eventshub.InputEvent(eventNoAudience))(ctx, t)
	})

	f.Assert("JobSink has audience in address", func(ctx context.Context, t feature.T) {
		gvk := schema.GroupVersionKind{
			Group:   jobsink.GVR().Group,
			Version: jobsink.GVR().Version,
			Kind:    "JobSink",
		}
		addressable.ValidateAddress(jobsink.GVR(), jobSink, addressable.AssertAddressWithAudience(
			auth.GetAudienceDirect(gvk, environment.FromContext(ctx).Namespace(), jobSink)),
		)(ctx, t)
	})
	f.Assert("Source sent the event with audience", assert.OnStore(source).
		Match(assert.MatchKind(eventshub.EventResponse)).
		Match(assert.MatchStatusCode(202)).
		AtLeast(1),
	)
	f.Assert("Source sent the event without audience", assert.OnStore(sourceNoAudience).
		Match(assert.MatchKind(eventshub.EventResponse)).
		Match(assert.MatchStatusCode(401)).
		AtLeast(1),
	)
	f.Assert("Job is created with the mounted event", assert.OnStore(sink).
		MatchReceivedEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("At least one Job is complete", AtLeastOneJobIsComplete(jobSink))

	return f
}

func AtLeastOneJobIsComplete(jobSinkName string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		interval, timeout := environment.PollTimingsFromContext(ctx)

		var jobs *batchv1.JobList
		err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (done bool, err error) {
			jobs, err = kubeclient.Get(ctx).BatchV1().
				Jobs(environment.FromContext(ctx).Namespace()).
				List(ctx, metav1.ListOptions{
					LabelSelector: fmt.Sprintf("%s=%s", sinks.JobSinkNameLabel, jobSinkName),
				})
			if err != nil {
				return false, fmt.Errorf("failed to get jobs: %w", err)
			}
			if len(jobs.Items) == 0 {
				t.Logf("Found no jobs associated with jobsink %q", jobSinkName)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			t.Errorf("failed to wait for job associated with jobsink to be present: %v", err)
			return
		}

		for _, j := range jobs.Items {
			if err := k8s.WaitUntilJobDone(ctx, t, j.Name); err == nil {
				// At least one job is done
				return
			}
		}

		jobs, err = kubeclient.Get(ctx).BatchV1().
			Jobs(environment.FromContext(ctx).Namespace()).
			List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", sinks.JobSinkNameLabel, jobSinkName),
			})
		if err != nil {
			t.Errorf("Failed to list jobs: %v", err)
			return
		}

		bytes, _ := json.MarshalIndent(jobs.Items, "", "  ")
		t.Errorf("No job is complete:\n%v", string(bytes))
	}
}
