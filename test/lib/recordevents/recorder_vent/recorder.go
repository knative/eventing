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

package recorder_vent

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/test/lib/recordevents"
)

type recorder struct {
	ctx       context.Context
	namespace string
	agentName string

	ref *corev1.ObjectReference
}

func (r *recorder) Vent(observed recordevents.EventInfo) error {
	b, err := json.Marshal(observed)
	if err != nil {
		return err
	}
	message := string(b)

	t := time.Now()
	// Note: DO NOT SET EventTime, or you'll trigger k8s api server hilarity:
	// - https://github.com/kubernetes/kubernetes/issues/95913
	// - https://github.com/kubernetes/kubernetes/blob/master/pkg/apis/core/validation/events.go#L122
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%v.%d", r.ref.Name, observed.Kind, observed.Sequence),
			Namespace: r.namespace,
		},
		InvolvedObject: *r.ref,
		Reason:         recordevents.CloudEventObservedReason,
		Message:        message,
		Source:         corev1.EventSource{Component: r.agentName},
		FirstTimestamp: metav1.Time{Time: t},
		LastTimestamp:  metav1.Time{Time: t},
		Count:          1,
		Type:           corev1.EventTypeNormal,
	}

	return r.recordEvent(event)
}

func (r *recorder) recordEvent(event *corev1.Event) error {
	tries := 0
	for {
		done, err := r.trySendEvent(event)
		if done {
			return nil
		}
		tries++
		if tries >= maxRetry {
			logging.FromContext(r.ctx).Errorf("Unable to write event '%s' (retry limit exceeded!)", event.Name)
			return err
		}
		// Randomize the first sleep so that various clients won't all be
		// synced up if the master goes down.
		if tries == 1 {
			time.Sleep(time.Duration(float64(sleepDuration) * rand.Float64()))
		} else {
			time.Sleep(sleepDuration)
		}
	}
}

func (r *recorder) trySendEvent(event *corev1.Event) (bool, error) {
	newEv, err := kubeclient.Get(r.ctx).CoreV1().Events(r.namespace).CreateWithEventNamespace(event)
	if err == nil {
		logging.FromContext(r.ctx).Infof("Event '%s' sent correctly, uuid: %s", newEv.Name, newEv.UID)
		return true, nil
	}

	// If we can't contact the server, then hold everything while we keep trying.
	// Otherwise, something about the event is malformed and we should abandon it.
	switch err.(type) {
	case *restclient.RequestConstructionError:
		// We will construct the request the same next time, so don't keep trying.
		logging.FromContext(r.ctx).Errorf("Unable to construct event '%s': '%v' (will not retry!)", event.Name, err)
		return true, err
	case *apierrors.StatusError:
		logging.FromContext(r.ctx).Errorf("Server rejected event '%s'. Reason: '%v' (will not retry!). Event: %v", event.Name, err, event)
		return true, err
	case *apierrors.UnexpectedObjectError:
		// We don't expect this; it implies the server's response didn't match a
		// known pattern. Go ahead and retry.
	default:
		// This case includes actual http transport errors. Go ahead and retry.
	}
	logging.FromContext(r.ctx).Errorf("Unable to write event: '%v' (may retry after sleeping)", err)
	return false, err
}
