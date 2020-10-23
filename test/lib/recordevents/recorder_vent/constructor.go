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
	"strings"

	"github.com/kelseyhightower/envconfig"
	"knative.dev/pkg/system"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
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
		eventBroadcaster := record.NewBroadcasterWithCorrelatorOptions(record.CorrelatorOptions{
			KeyFunc: func(event *corev1.Event) (aggregateKey string, localKey string) {
				return strings.Join([]string{
					event.Source.Component,
					event.Source.Host,
					event.InvolvedObject.Kind,
					event.InvolvedObject.Namespace,
					event.InvolvedObject.Name,
					string(event.InvolvedObject.UID),
					event.InvolvedObject.APIVersion,
					event.Type,
					event.Reason,
				}, ""), string(event.UID)
			},
		})
		watches := []watch.Interface{
			eventBroadcaster.StartLogging(logger.Named("event-broadcaster").Infof),
			eventBroadcaster.StartRecordingToSink(
				&v1.EventSinkImpl{Interface: kubeclient.Get(ctx).CoreV1().Events(system.Namespace())},
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
