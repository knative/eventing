// +build e2e

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

package e2e

import (
	"fmt"
	"testing"

	. "github.com/cloudevents/sdk-go/v2/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"

	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"

	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
)

func TestContainerSourceV1Alpha2(t *testing.T) {
	const (
		containerSourceName = "e2e-container-source"
		templateName        = "e2e-container-source-template"
		// the heartbeats image is built from test_images/heartbeats
		imageName = "heartbeats"

		recordEventPodName = "e2e-container-source-logger-pod"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event record pod
	eventTracker, _ := recordevents.StartEventRecordOrFail(client, recordEventPodName)
	// create container source
	message := fmt.Sprintf("TestContainerSource%s", uuid.NewUUID())
	// args are the arguments passing to the container, msg is used in the heartbeats image
	args := []string{"--msg=" + message}
	// envVars are the environment variables of the container
	envVars := []corev1.EnvVar{{
		Name:  "POD_NAME",
		Value: templateName,
	}, {
		Name:  "POD_NAMESPACE",
		Value: client.Namespace,
	}}
	containerSource := eventingtesting.NewContainerSourceV1Alpha2(
		containerSourceName,
		client.Namespace,
		eventingtesting.WithContainerSourceSpecV1A2(sourcesv1alpha2.ContainerSourceSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: templateName,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            imageName,
						Image:           pkgTest.ImagePath(imageName),
						ImagePullPolicy: corev1.PullAlways,
						Args:            args,
						Env:             envVars,
					}},
				},
			},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{Ref: resources.KnativeRefForService(recordEventPodName, client.Namespace)},
			},
		}),
	)
	client.CreateContainerSourceV1Alpha2OrFail(containerSource)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail()

	// verify the logger service receives the event
	eventTracker.AssertAtLeast(2, recordevents.MatchEvent(
		recordevents.MatchHeartBeatsImageMessage(message),
		HasSource(fmt.Sprintf("https://knative.dev/eventing/test/heartbeats/#%s/%s", client.Namespace, templateName)),
	))
}
