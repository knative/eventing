/*
Copyright 2018 The Knative Authors

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

package sources

import (
	"encoding/base64"
	"encoding/json"

	v1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FeedOperation specifies whether we're starting or stopping
type FeedOperation string

const (
	// Each feed lifecycle pod gets these.
	watcherContainerCPU = "400m"

	// START specifies a Feed should be started
	StartFeed FeedOperation = "START"

	// STOP specifies a Feed should be stopped
	StopFeed FeedOperation = "STOP"

	// FeedOperationKey is the Env variable that gets set to requested FeedOperationKey
	FeedOperationKey string = "FEED_OPERATION"

	// FeedTriggerKey is the Env variable that gets set to serialized trigger configuration
	FeedTriggerKey string = "FEED_TRIGGER"

	// FeedTargetKey is the Env variable that gets set to target of the feed
	FeedTargetKey string = "FEED_TARGET"

	// FeedContextKey is the Env variable that gets set to serialized FeedContext if stopping
	FeedContextKey string = "FEED_CONTEXT"

	// EventSourceParametersKey is the Env variable that gets set to serialized EventSourceSpec
	EventSourceParametersKey string = "EVENT_SOURCE_PARAMETERS"

	// FeedNamespaceKey is the Env variable that gets set to namespace of the container performing
	// the Feed lifecycle operation (aka, namespace of the feed). Uses downward api
	FeedNamespaceKey string = "FEED_NAMESPACE"

	// FeedServiceAccount is the Env variable that gets set to serviceaccount of the
	// container performing the Feed operation. Uses downward api
	FeedServiceAccountKey string = "FEED_SERVICE_ACCOUNT"
)

// MakeJob creates a Job to complete a start or stop operation for a Feed.
func MakeJob(feed *v1alpha1.Feed, namespace string, serviceAccountName string, jobName string, spec *v1alpha1.EventSourceSpec, op FeedOperation, trigger EventTrigger, route string, feedContext FeedContext) (*batchv1.Job, error) {
	labels := map[string]string{
		"app": "feedlifecyclepod",
	}

	podTemplate, err := makePodTemplate(feed, namespace, serviceAccountName, jobName, spec, op, trigger, route, feedContext)
	if err != nil {
		return nil, err
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            jobName,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(feed, v1alpha1.SchemeGroupVersion.WithKind("Feed"))},
		},
		Spec: batchv1.JobSpec{
			Template: *podTemplate,
		},
	}, nil
}

// makePodTemplate creates a pod template for starting or stopping a feed.
func makePodTemplate(feed *v1alpha1.Feed, namespace string, serviceAccountName string, podName string, spec *v1alpha1.EventSourceSpec, op FeedOperation, trigger EventTrigger, route string, feedContext FeedContext) (*corev1.PodTemplateSpec, error) {
	marshalledFeedContext, err := json.Marshal(feedContext)
	if err != nil {
		return nil, err
	}
	encodedFeedContext := base64.StdEncoding.EncodeToString(marshalledFeedContext)

	marshalledTrigger, err := json.Marshal(trigger)
	if err != nil {
		return nil, err
	}
	encodedTrigger := base64.StdEncoding.EncodeToString(marshalledTrigger)

	encodedParameters := ""
	if spec.Parameters != nil {
		marshalledParameters, err := json.Marshal(spec.Parameters)
		if err != nil {
			return nil, err
		}
		encodedParameters = base64.StdEncoding.EncodeToString(marshalledParameters)
	}

	return &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "false",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: serviceAccountName,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				corev1.Container{
					Name:            podName,
					Image:           spec.Image,
					ImagePullPolicy: "Always",
					Env: []corev1.EnvVar{
						{
							Name:  FeedOperationKey,
							Value: string(op),
						},
						{
							Name:  FeedTargetKey,
							Value: route,
						},
						{
							Name:  FeedTriggerKey,
							Value: encodedTrigger,
						},
						{
							Name:  FeedContextKey,
							Value: encodedFeedContext,
						},
						{
							Name:  EventSourceParametersKey,
							Value: encodedParameters,
						},
						{
							Name: FeedNamespaceKey,
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.namespace",
								},
							},
						},
						{
							Name: FeedServiceAccountKey,
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "spec.serviceAccountName",
								},
							},
						},
					},
				},
			},
		},
	}, nil
}
