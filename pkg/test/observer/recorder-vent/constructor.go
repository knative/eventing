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
	"log"
	"os"

	"github.com/kelseyhightower/envconfig"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/test/observer"
)

type envConfig struct {
	AgentName string `envconfig:"AGENT_NAME" default:"observer-default" required:"true"`
	EventOn   string `envconfig:"K8S_EVENT_SINK" required:"true"`

	Port int    `envconfig:"PORT" default:"8080" required:"true"`
	Sink string `envconfig:"K_SINK"`
}

func NewFromEnv(ctx context.Context) observer.EventLog {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process env var: %s", err)
		os.Exit(1)
	}

	var ref duckv1.KReference
	if err := json.Unmarshal([]byte(env.EventOn), &ref); err != nil {
		log.Printf("[ERROR] Failed to process env var [K8S_EVENT_SINK]: %s", err)
		os.Exit(1)
	}

	return NewEventLog(ctx, env.AgentName, ref)
}

func NewEventLog(ctx context.Context, agentName string, ref duckv1.KReference) observer.EventLog {

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		logging.FromContext(ctx).Fatalf("failed to parse group version, %s", err)
	}

	gvr, _ := meta.UnsafeGuessKindToResource(gv.WithKind(ref.Kind))

	var on runtime.Object
	if ref.Namespace == "" {
		on, err = dynamicclient.Get(ctx).Resource(gvr).Get(ctx, ref.Name, metav1.GetOptions{})
	} else {
		on, err = dynamicclient.Get(ctx).Resource(gvr).Get(ctx, ref.Name, metav1.GetOptions{})
	}
	if err != nil {
		logging.FromContext(ctx).Fatalf("failed to fetch object ref, %+v, %s", ref, err)

	}

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
			eventBroadcaster.StartRecordingToSink(
				&v1.EventSinkImpl{Interface: kubeclient.Get(ctx).CoreV1().Events("")}),
		}
		recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: agentName})
		go func() {
			<-ctx.Done()
			for _, w := range watches {
				w.Stop()
			}
		}()
	}

	return recorder
}
