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
	"time"

	"github.com/kelseyhightower/envconfig"
	"knative.dev/pkg/system"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ref "k8s.io/client-go/tools/reference"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
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

	reference, err := ref.GetReference(scheme.Scheme, on)
	if err != nil {
		logging.FromContext(ctx).Fatalf("Could not construct reference to: '%#v' due to: '%v'", on, err)
	}

	return &recorder{
		ctx:       ctx,
		namespace: system.Namespace(),
		agentName: agentName,
		ref:       reference,
	}
}
