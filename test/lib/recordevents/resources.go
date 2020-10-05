package recordevents

import (
	"fmt"

	"k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/pkg/test"
)

// EventRecordPod creates a Pod that stores received events for test retrieval.
func EventRecordPod(name string, namespace string, serviceAccountName string) *v1.Pod {
	return recordEventsPod("recordevents", name, namespace, serviceAccountName)
}

func recordEventsPod(imageName string, name string, namespace string, serviceAccountName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:            imageName,
				Image:           test.ImagePath(imageName),
				ImagePullPolicy: v1.PullAlways,
				Env: []v1.EnvVar{{
					Name:  "SYSTEM_NAMESPACE",
					Value: namespace,
				}, {
					Name:  "OBSERVER",
					Value: "recorder-" + name,
				}, {
					Name:  "K8S_EVENT_SINK",
					Value: fmt.Sprintf("{\"apiVersion\": \"v1\", \"kind\": \"Pod\", \"name\": \"%s\", \"namespace\": \"%s\"}", name, namespace),
				}},
			}},
			ServiceAccountName: serviceAccountName,
			RestartPolicy:      v1.RestartPolicyAlways,
		},
	}
}
