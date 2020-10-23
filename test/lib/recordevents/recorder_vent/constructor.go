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
	"log"
	"math/rand"
	"time"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/apimachinery/pkg/api/errors"
	restclient "k8s.io/client-go/rest"
	"knative.dev/pkg/system"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/test/lib/recordevents"
)

type envConfig struct {
	AgentName string `envconfig:"AGENT_NAME" default:"observer-default" required:"true"`
	PodName   string `envconfig:"POD_NAME" required:"true"`
	Port      int    `envconfig:"PORT" default:"8080" required:"true"`
}

const (
	maxRetry      = 5
	sleepDuration = 5 * time.Second
)

func NewFromEnv(ctx context.Context) recordevents.EventLog {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", err)
	}

	logging.FromContext(ctx).Infof("Recorder vent environment configuration: %+v", env)

	return NewEventLog(ctx, env.AgentName, env.PodName)
}

func NewEventLog(ctx context.Context, agentName string, podName string) recordevents.EventLog {
	on, err := kubeclient.Get(ctx).CoreV1().Pods(system.Namespace()).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		logging.FromContext(ctx).Fatal("Error while trying to retrieve the pod", err)
	}

	logging.FromContext(ctx).Infof("Going to send events to pod '%s' in namespace '%s'", on.Name, on.Namespace)

	return &recorder{out: createRecorder(ctx, agentName), on: on}
}

func createRecorder(ctx context.Context, agentName string) record.EventRecorder {
	logger := logging.FromContext(ctx)

	recorder := controller.GetEventRecorder(ctx)
	if recorder == nil {
		// Create event broadcaster
		logger.Debug("Creating event broadcaster")
		eventBroadcaster := record.NewBroadcaster()
		watches := []watch.Interface{
			eventBroadcaster.StartLogging(logger.Named("event-broadcaster").Infof),
			eventBroadcaster.StartEventWatcher(
				sendToSink(ctx, kubeclient.Get(ctx).CoreV1().Events(system.Namespace()).CreateWithEventNamespace),
			),
		}
		recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: agentName})
		go func() {
			<-ctx.Done()
			for _, w := range watches {
				w.Stop()
			}
			logging.FromContext(ctx).Debug("Closed event-broadcaster")
		}()
	}

	return recorder
}

func sendToSink(ctx context.Context, sender func(*corev1.Event) (*corev1.Event, error)) func(*corev1.Event) {
	return func(event *corev1.Event) {
		tries := 0
		for {
			if recordEvent(ctx, sender, event) {
				break
			}
			tries++
			if tries >= maxRetry {
				logging.FromContext(ctx).Errorf("Unable to write event '%s' (retry limit exceeded!)", event.Name)
				break
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
}

func recordEvent(ctx context.Context, sender func(*corev1.Event) (*corev1.Event, error), event *corev1.Event) bool {
	newEv, err := sender(event)
	if err == nil {
		logging.FromContext(ctx).Infof("Event '%s' sent correctly, uuid: %s", newEv.Name, newEv.UID)
		return true
	}

	// If we can't contact the server, then hold everything while we keep trying.
	// Otherwise, something about the event is malformed and we should abandon it.
	switch err.(type) {
	case *restclient.RequestConstructionError:
		// We will construct the request the same next time, so don't keep trying.
		logging.FromContext(ctx).Errorf("Unable to construct event '%s': '%v' (will not retry!)", event.Name, err)
		return true
	case *errors.StatusError:
		if errors.IsAlreadyExists(err) {
			logging.FromContext(ctx).Infof("Server rejected event '%s': '%v' (will not retry!)", event.Name, err)
		} else {
			logging.FromContext(ctx).Errorf("Server rejected event '%s': '%v' (will not retry!)", event.Name, err)
		}
		return true
	case *errors.UnexpectedObjectError:
		// We don't expect this; it implies the server's response didn't match a
		// known pattern. Go ahead and retry.
	default:
		// This case includes actual http transport errors. Go ahead and retry.
	}
	logging.FromContext(ctx).Errorf("Unable to write event: '%v' (may retry after sleeping)", err)
	return false
}
