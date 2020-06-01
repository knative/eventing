package sender

import (
	"encoding/json"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgTest "knative.dev/pkg/test"
)

type EventSenderOption func(*corev1.Pod)

// EnableTracing enables tracing in sender pod
func EnableTracing() EventSenderOption {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Args = append(
			pod.Spec.Containers[0].Args,
			"-add-tracing",
			"true",
		)
	}
}

// EnableIncrementalId creates a new incremental id for each event sent from the sender pod
func EnableIncrementalId() EventSenderOption {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Args = append(
			pod.Spec.Containers[0].Args,
			"-incremental-id",
			"true",
		)
	}
}

// WithEncoding forces the encoding of the event to send from the sender pod
func WithEncoding(encoding cloudevents.Encoding) EventSenderOption {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Args = append(
			pod.Spec.Containers[0].Args,
			"-event-encoding",
			encoding.String(),
		)
	}
}

// EventSenderPod creates a Pod that sends events to the given address.
func EventSenderPod(imageName string, name string, sink string, event cloudevents.Event, options ...EventSenderOption) (*corev1.Pod, error) {
	encodedEvent, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	args := []string{
		"-sink",
		sink,
		"-event",
		string(encodedEvent),
	}

	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
				Args:            args,
			}},
			// Never restart the event sender Pod.
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	for _, opt := range options {
		opt(p)
	}

	return p, nil
}
