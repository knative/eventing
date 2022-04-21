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

package setupclientoptions

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"k8s.io/apimachinery/pkg/util/uuid"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	sourcesv1beta2 "knative.dev/eventing/pkg/apis/sources/v1beta2"
	eventingtestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
	eventingtestingv1beta2 "knative.dev/eventing/pkg/reconciler/testing/v1beta2"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// ApiServerSourceV1ClientSetupOption returns a ClientSetupOption that can be used
// to create a new ApiServerSource. It creates a ServiceAccount, a Role, a
// RoleBinding, a RecordEvents pod and an ApiServerSource object with the event
// mode and RecordEvent pod as its sink.
func ApiServerSourceV1ClientSetupOption(ctx context.Context, name string, mode string,
	recordEventsPodName string) testlib.SetupClientOption {
	return func(client *testlib.Client) {
		// create event record
		recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)

		spec := sourcesv1.ApiServerSourceSpec{
			Resources: []sourcesv1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Event",
			}},
			EventMode:          mode,
			ServiceAccountName: client.Namespace + "-eventwatcher",
		}
		spec.Sink = duckv1.Destination{Ref: resources.ServiceKRef(recordEventsPodName)}

		apiServerSource := eventingtestingv1.NewApiServerSource(
			name,
			client.Namespace,
			eventingtestingv1.WithApiServerSourceSpec(spec),
		)

		client.CreateApiServerSourceV1OrFail(apiServerSource)

		// wait for all test resources to be ready
		client.WaitForAllTestResourcesReadyOrFail(ctx)
	}
}

// PingSourceV1B2ClientSetupOption returns a ClientSetupOption that can be used
// to create a new PingSource. It creates a RecordEvents pod and a
// PingSource object with the RecordEvent pod as its sink.
func PingSourceV1B2ClientSetupOption(ctx context.Context, name string, recordEventsPodName string) testlib.SetupClientOption {
	return func(client *testlib.Client) {

		// create event logger pod and service
		recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)

		// create cron job source
		data := fmt.Sprintf(`{"msg":"TestPingSource %s"}`, uuid.NewUUID())
		source := eventingtestingv1beta2.NewPingSource(
			name,
			client.Namespace,
			eventingtestingv1beta2.WithPingSourceSpec(sourcesv1beta2.PingSourceSpec{
				ContentType: cloudevents.ApplicationJSON,
				Data:        data,
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: resources.KnativeRefForService(recordEventsPodName, client.Namespace),
					},
				},
			}),
		)
		client.CreatePingSourceV1Beta2OrFail(source)

		// wait for all test resources to be ready
		client.WaitForAllTestResourcesReadyOrFail(ctx)
	}
}
