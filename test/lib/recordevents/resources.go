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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	pkgtest "knative.dev/pkg/test"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
)

type EventRecordOption = func(*corev1.Pod, *testlib.Client) error

// EchoEvent is an option to let the recordevents reply with the received event
func EchoEvent(pod *corev1.Pod, client *testlib.Client) error {
	pod.Spec.Containers[0].Env = append(
		pod.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "REPLY", Value: "true"},
	)
	return nil
}

var _ EventRecordOption = EchoEvent

// ReplyWithTransformedEvent is an option to let the recordevents reply with the transformed event
func ReplyWithTransformedEvent(replyEventType string, replyEventSource string, replyEventData string) EventRecordOption {
	return func(pod *corev1.Pod, client *testlib.Client) error {
		pod.Spec.Containers[0].Env = append(
			pod.Spec.Containers[0].Env,
			corev1.EnvVar{Name: "REPLY", Value: "true"},
		)
		if replyEventType != "" {
			pod.Spec.Containers[0].Env = append(
				pod.Spec.Containers[0].Env,
				corev1.EnvVar{Name: "REPLY_EVENT_TYPE", Value: replyEventType},
			)
		}
		if replyEventSource != "" {
			pod.Spec.Containers[0].Env = append(
				pod.Spec.Containers[0].Env,
				corev1.EnvVar{Name: "REPLY_EVENT_SOURCE", Value: replyEventSource},
			)
		}
		if replyEventData != "" {
			pod.Spec.Containers[0].Env = append(
				pod.Spec.Containers[0].Env,
				corev1.EnvVar{Name: "REPLY_EVENT_DATA", Value: replyEventData},
			)
		}

		return nil
	}
}

// ReplyWithAppendedData is an option to let the recordevents reply with the transformed event with appended data
func ReplyWithAppendedData(appendData string) EventRecordOption {
	return func(pod *corev1.Pod, client *testlib.Client) error {
		pod.Spec.Containers[0].Env = append(
			pod.Spec.Containers[0].Env,
			corev1.EnvVar{Name: "REPLY", Value: "true"},
		)
		if appendData != "" {
			pod.Spec.Containers[0].Env = append(
				pod.Spec.Containers[0].Env,
				corev1.EnvVar{Name: "REPLY_APPEND_DATA", Value: appendData},
			)
		}

		return nil
	}
}

// DeployEventRecordOrFail deploys the recordevents image with necessary sa, roles, rb to execute the image
func DeployEventRecordOrFail(ctx context.Context, client *testlib.Client, name string, options ...EventRecordOption) *corev1.Pod {
	client.CreateServiceAccountOrFail(name)
	client.CreateRoleOrFail(resources.Role(name,
		resources.WithRuleForRole(&rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get"},
		}),
		resources.WithRuleForRole(&rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{rbacv1.VerbAll},
		}),
	))
	client.CreateRoleBindingOrFail(name, "Role", name, name, client.Namespace)

	eventRecordPod := recordEventsPod("recordevents", name, name)
	client.CreatePodOrFail(eventRecordPod, append(options, testlib.WithService(name))...)
	err := pkgtest.WaitForPodRunning(ctx, client.Kube, name, client.Namespace)
	if err != nil {
		client.T.Fatalf("Failed to start the recordevent pod '%s': %v", name, errors.WithStack(err))
	}
	client.WaitForServiceEndpointsOrFail(ctx, name, 1)
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
				}},
			}},
			ServiceAccountName: serviceAccountName,
			RestartPolicy:      corev1.RestartPolicyAlways,
		},
	}
}
