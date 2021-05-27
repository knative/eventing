// +build e2e

/*
Copyright 2021 The Knative Authors

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

package e2e

import (
	"testing"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	testhelpers "knative.dev/pkg/test/helpers"
)

func TestTriggerNotReadyWithoutSubscriber(t *testing.T) {
	for _, testCase := range triggerNotReadyWithoutSubscriberTestCases() {
		refFn := testCase.refFn
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.skipped != "" {
				t.Skip(testCase.skipped)
			}
			brokerName := testhelpers.MakeK8sNamePrefix(t.Name())
			client := testlib.Setup(t, true)
			defer testlib.TearDown(client)
			client.CreateBrokerOrFail(brokerName)

			name := testhelpers.MakeK8sNamePrefix(testCase.name)
			ref := refFn(name, client.Namespace)
			dest := duckv1.Destination{Ref: ref}
			subscriberOption := resources.WithSubscriberDestination(func(t *eventingv1.Trigger) duckv1.Destination {
				return dest
			})

			trgr := client.CreateTriggerOrFail(
				name,
				resources.WithBroker(brokerName),
				resources.WithAttributesTriggerFilter(
					eventingv1.TriggerAnyFilter,
					"org.example.not-important",
					map[string]interface{}{},
				),
				subscriberOption,
			)
			meta := resources.NewMetaResource(
				trgr.Name, trgr.Namespace, testlib.TriggerTypeMeta,
			)
			err := duck.WaitForResourceReady(client.Dynamic, meta)
			if err == nil {
				t.Fatal("trigger is ready, but shouldn't be without subscriber being deployed")
			}
		})
	}
}

func triggerNotReadyWithoutSubscriberTestCases() []triggerNotReadyWithoutSubscriberTestCase {
	return []triggerNotReadyWithoutSubscriberTestCase{{
		name:  "K8s Service",
		refFn: resources.KnativeRefForService,
		skipped: "knative/eventing#5442 Regular, non existing, K8s service as " +
			"subscriber causes trigger to report Ready incorrectly",
	}, {
		name:  "Knative Service",
		refFn: resources.KnativeRefForKservice,
	}}
}

type triggerNotReadyWithoutSubscriberTestCase struct {
	name    string
	refFn   func(name, namespace string) *duckv1.KReference
	skipped string
}
