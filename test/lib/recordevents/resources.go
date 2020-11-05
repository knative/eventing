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
	"encoding/json"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
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

func noop(pod *corev1.Pod, client *testlib.Client) error {
	return nil
}

func compose(options ...EventRecordOption) EventRecordOption {
	return func(pod *corev1.Pod, client *testlib.Client) error {
		for _, opt := range options {
			if err := opt(pod, client); err != nil {
				return err
			}
		}
		return nil
	}
}

func envOptionalOpt(key, value string) EventRecordOption {
	if value != "" {
		return func(pod *corev1.Pod, client *testlib.Client) error {
			pod.Spec.Containers[0].Env = append(
				pod.Spec.Containers[0].Env,
				corev1.EnvVar{Name: key, Value: value},
			)
			return nil
		}
	} else {
		return noop
	}
}

func envOption(key, value string) EventRecordOption {
	return func(pod *corev1.Pod, client *testlib.Client) error {
		pod.Spec.Containers[0].Env = append(
			pod.Spec.Containers[0].Env,
			corev1.EnvVar{Name: key, Value: value},
		)
		return nil
	}
}

// EchoEvent is an option to let the recordevents reply with the received event
var EchoEvent EventRecordOption = envOption("REPLY", "true")

// ReplyWithTransformedEvent is an option to let the recordevents reply with the transformed event
func ReplyWithTransformedEvent(replyEventType string, replyEventSource string, replyEventData string) EventRecordOption {
	return compose(
		envOption("REPLY", "true"),
		envOptionalOpt("REPLY_EVENT_TYPE", replyEventType),
		envOptionalOpt("REPLY_EVENT_SOURCE", replyEventSource),
		envOptionalOpt("REPLY_EVENT_DATA", replyEventData),
	)
}

// ReplyWithAppendedData is an option to let the recordevents reply with the transformed event with appended data
func ReplyWithAppendedData(appendData string) EventRecordOption {
	return compose(
		envOption("REPLY", "true"),
		envOptionalOpt("REPLY_APPEND_DATA", appendData),
	)
}

// InputEvent is an option to provide the event to send when deploying the event sender
func InputEvent(event cloudevents.Event) EventRecordOption {
	encodedEvent, err := json.Marshal(event)
	if err != nil {
		return func(pod *corev1.Pod, client *testlib.Client) error {
			return err
		}
	}
	return envOption("INPUT_EVENT", string(encodedEvent))
}

// AddTracing adds tracing headers when sending events.
func AddTracing() EventRecordOption {
	return envOption("ADD_TRACING", "true")
}

// EnableIncrementalId creates a new incremental id for each sent event.
func EnableIncrementalId() EventRecordOption {
	return envOption("INCREMENTAL_ID", "true")
}

// InputEncoding forces the encoding of the event for each sent event.
func InputEncoding(encoding cloudevents.Encoding) EventRecordOption {
	return envOption("EVENT_ENCODING", encoding.String())
}

// InputHeaders adds the following headers to the sent requests.
func InputHeaders(headers map[string]string) EventRecordOption {
	return envOption("INPUT_HEADERS", serializeHeaders(headers))
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

	options = append(
		options,
		testlib.WithService(name),
		envOption("EVENT_GENERATORS", "receiver"),
	)

	eventRecordPod := recordEventsPod("recordevents", name, name)
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

	options = append(
		options,
		envOption("EVENT_GENERATORS", "sender"),
		envOption("SINK", sink),
	)

	eventRecordPod := recordEventsPod("recordevents", name, name)
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
