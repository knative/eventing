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

package recordevents

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	pkgtest "knative.dev/pkg/test"

	testlib "knative.dev/eventing/test/lib"
)

func noop(pod *corev1.Pod, client *testlib.Client) error {
	return nil
}

func serializeHeaders(headers map[string]string) string {
	kv := make([]string, 0, len(headers))
	for k, v := range headers {
		kv = append(kv, k+":"+v)
	}
	return strings.Join(kv, ",")
}

// DeployEventRecordOrFail deploys the recordevents image with necessary sa, roles, rb to execute the image
func DeployEventRecordOrFail(ctx context.Context, client *testlib.Client, name string, options ...EventRecordOption) *corev1.Pod {
	options = append(
		options,
		testlib.WithService(name),
		envOption("EVENT_GENERATORS", "receiver"),
	)

	eventRecordPod := recordEventsPod("recordevents", name, client.Namespace)
	client.CreatePodOrFail(eventRecordPod, options...)
	err := pkgtest.WaitForPodRunning(ctx, client.Kube, name, client.Namespace)
	if err != nil {
		client.T.Fatalf("Failed to start the recordevent pod '%s': %v", name, errors.WithStack(err))
	}
	client.WaitForServiceEndpointsOrFail(ctx, name, 1)
	return eventRecordPod
}

// DeployEventSenderOrFail deploys the recordevents image with necessary sa, roles, rb to execute the image
func DeployEventSenderOrFail(ctx context.Context, client *testlib.Client, name string, sink string, options ...EventRecordOption) *corev1.Pod {
	options = append(
		options,
		envOption("EVENT_GENERATORS", "sender"),
		envOption("SINK", sink),
	)

	eventRecordPod := recordEventsPod("recordevents", name, client.Namespace)
	client.CreatePodOrFail(eventRecordPod, options...)
	err := pkgtest.WaitForPodRunning(ctx, client.Kube, name, client.Namespace)
	if err != nil {
		client.T.Fatalf("Failed to start the recordevent pod '%s': %v", name, errors.WithStack(err))
	}
	return eventRecordPod
}

func recordEventsPod(imageName string, name string, serviceAccountName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgtest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{{
					Name: "SYSTEM_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
					},
				}, {
					Name: "POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
					},
				}, {
					Name:  "EVENT_LOGS",
					Value: "recorder,logger",
				}},
			}},
			ServiceAccountName: serviceAccountName,
			RestartPolicy:      corev1.RestartPolicyAlways,
		},
	}
}
