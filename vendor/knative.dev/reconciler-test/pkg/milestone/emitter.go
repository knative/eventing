/*
Copyright 2021 The Knative Authors

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

package milestone

import (
	"context"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"

	"knative.dev/reconciler-test/pkg/feature"
)

const (
	enableRetry  = true
	retryBackoff = 10 * time.Millisecond
	maxTries     = 3
)

type envConfig struct {
	MilestoneEventsTarget string `envconfig:"MILESTONE_EVENTS_TARGET"`
}

// Emitter sends milestone events.
type Emitter interface {
	// Helpers to create known events.

	Environment(env map[string]string)
	NamespaceCreated(namespace string)
	NamespaceDeleted(namespace string)
	TestStarted(feature string, t feature.T)
	TestFinished(feature string, t feature.T)
	StepsPlanned(feature string, steps map[feature.Timing][]feature.Step, t feature.T)
	StepStarted(feature string, step *feature.Step, t feature.T)
	StepFinished(feature string, step *feature.Step, t feature.T)
	TestSetStarted(featureSet string, t feature.T)
	TestSetFinished(featureSet string, t feature.T)
	Finished(result Result)
	Exception(reason, messageFormat string, messageA ...interface{})
}

type Result interface {
	Failed() bool
}

// NewMilestoneEmitterFromEnv will attempt to pull the env var
// `MILESTONE_EVENTS_TARGET` as the target uri for sending milestone events.
func NewMilestoneEmitterFromEnv(instance, namespace string) (Emitter, error) {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		return nil, err
	}
	if len(env.MilestoneEventsTarget) > 0 {
		fmt.Printf("milestone events target: %s\n\n", env.MilestoneEventsTarget)
		return NewMilestoneEmitter(instance, namespace, env.MilestoneEventsTarget)
	}
	return &NilSafeClient{}, nil
}

// NewMilestoneEmitter will convert target uri to a milestone event sender and return it.
func NewMilestoneEmitter(instance, namespace, uri string) (Emitter, error) {
	target, err := apis.ParseURL(uri)
	if err != nil {
		return nil, err
	}
	switch target.Scheme {
	case "http", "https":
		p, err := cloudevents.NewHTTP(cloudevents.WithTarget(target.String()))
		if err != nil {
			return nil, err
		}
		client, err := cloudevents.NewClient(p, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
		return &NilSafeClient{
			Client:  client,
			Factory: NewFactory(instance, namespace),
		}, err
	default:
		return nil, fmt.Errorf("unsupported milestone event target uri: %q", target.String())
	}
}

// NilSafeClient is a simple wrapper around a cloudevent client that implements
// Emitter to provide nil check safety.
type NilSafeClient struct {
	Client  cloudevents.Client
	Factory *Factory
}

func (n *NilSafeClient) Environment(env map[string]string) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.Environment(env))
}

func (n *NilSafeClient) NamespaceCreated(namespace string) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.NamespaceCreated(namespace))
}

func (n *NilSafeClient) NamespaceDeleted(namespace string) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.NamespaceDeleted(namespace))
}

func (n *NilSafeClient) TestStarted(feature string, t feature.T) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.TestStarted(feature, t.Name()))
}

func (n *NilSafeClient) TestFinished(feature string, t feature.T) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.TestFinished(feature, t.Name(), t.Skipped(), t.Failed()))
}

func (n *NilSafeClient) StepsPlanned(feature string, steps map[feature.Timing][]feature.Step, t feature.T) {
	if n == nil || n.Client == nil {
		return
	}

	sm := make(map[string][]string)
	for k, v := range steps {
		sm[k.String()] = make([]string, len(v))
		for i, step := range v {
			sm[k.String()][i] = step.Name
		}
	}

	n.Event(context.Background(), n.Factory.StepsPlanned(feature, sm, t.Name()))
}

func (n *NilSafeClient) StepStarted(feature string, step *feature.Step, t feature.T) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.StepStarted(feature, step.Name, step.T.String(), step.L.String(), t.Name()))
}

func (n *NilSafeClient) StepFinished(feature string, step *feature.Step, t feature.T) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.StepFinished(feature, step.Name, step.T.String(), step.L.String(), t.Name(), t.Skipped(), t.Failed()))
}

func (n *NilSafeClient) TestSetStarted(featureSet string, t feature.T) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.TestSetStarted(featureSet, t.Name()))
}

func (n *NilSafeClient) TestSetFinished(featureSet string, t feature.T) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.TestSetFinished(featureSet, t.Name(), t.Skipped(), t.Failed()))
}

func (n *NilSafeClient) Finished(_ Result) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.Finished())
}

func (n *NilSafeClient) Exception(reason, messageFormat string, messageA ...interface{}) {
	if n == nil || n.Client == nil {
		return
	}
	n.Event(context.Background(), n.Factory.Exception(reason, messageFormat, messageA...))
}

// Event implements Emitter.Event.
func (n *NilSafeClient) Event(ctx context.Context, event cloudevents.Event) {
	if n == nil || n.Client == nil {
		return
	}
	if enableRetry {
		// Adds retry to the outbound send attempt.
		ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, retryBackoff, maxTries)
	}
	if result := n.Client.Send(ctx, event); cloudevents.IsUndelivered(result) {
		logging.FromContext(ctx).Errorw("failed to deliver milestone event", zap.Error(result))
	} else if cloudevents.IsNACK(result) {
		logging.FromContext(ctx).Errorw("milestone event target returned NACK", zap.Error(result), zap.String("event", event.Type()))
	}
}
