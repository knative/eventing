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
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// ApiServerSourceClientSetupOption returns a ClientSetupOption that can be used
// to create a new ApiServerSource. It creates a ServiceAccount, a Role, a
// RoleBinding, a RecordEvents pod and an ApiServerSource object with the event
// mode and RecordEvent pod as its sink.
func ApiServerSourceClientSetupOption(name string, mode string, recordEventsPodName string,
	roleName string, serviceAccountName string) testlib.SetupClientOption {
	return func(client *testlib.Client) {
		// create needed RBAC SA, Role & RoleBinding
		createRbacObjects(client, roleName, serviceAccountName)

		// create event record
		recordevents.StartEventRecordOrFail(client,
			recordEventsPodName)

		spec := sourcesv1beta1.ApiServerSourceSpec{
			Resources: []sourcesv1beta1.APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Event",
			}},
			EventMode:          mode,
			ServiceAccountName: serviceAccountName,
		}
		spec.Sink = duckv1.Destination{Ref: resources.ServiceKRef(recordEventsPodName)}

		apiServerSource := eventingtesting.NewApiServerSourceV1Beta1(
			name,
			client.Namespace,
			eventingtesting.WithApiServerSourceSpecV1B1(spec),
		)

		client.CreateApiServerSourceV1Beta1OrFail(apiServerSource)

		// wait for all test resources to be ready
		client.WaitForAllTestResourcesReadyOrFail()
	}
}

// PingSourceClientSetupOption returns a ClientSetupOption that can be used
// to create a new PingSource. It creates a RecordEvents pod and a
// PingSource object with the RecordEvent pod as its sink.
func PingSourceClientSetupOption(name string, recordEventsPodName string) testlib.SetupClientOption {
	return func(client *testlib.Client) {

		// create event logger pod and service
		recordevents.StartEventRecordOrFail(client, recordEventsPodName)

		// create cron job source
		data := fmt.Sprintf(`{"msg":"TestPingSource %s"}`, uuid.NewUUID())
		source := eventingtesting.NewPingSourceV1Alpha2(
			name,
			client.Namespace,
			eventingtesting.WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
				JsonData: data,
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: resources.KnativeRefForService(recordEventsPodName, client.Namespace),
					},
				},
			}),
		)
		client.CreatePingSourceV1Alpha2OrFail(source)

		// wait for all test resources to be ready
		client.WaitForAllTestResourcesReadyOrFail()
	}
}

func createRbacObjects(client *testlib.Client, roleName string,
	serviceAccountName string) {
	// creates ServiceAccount and RoleBinding with a role for reading pods
	// and events
	r := resources.Role(roleName,
		resources.WithRuleForRole(&rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"events", "pods"},
			Verbs:     []string{"get", "list", "watch"}}))
	client.CreateServiceAccountOrFail(serviceAccountName)
	client.CreateRoleOrFail(r)
	client.CreateRoleBindingOrFail(
		serviceAccountName,
		testlib.RoleKind,
		roleName,
		fmt.Sprintf("%s-%s", serviceAccountName, roleName),
		client.Namespace,
	)
}
