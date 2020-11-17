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

package mtping

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	kncloudevents "knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
	"knative.dev/eventing/pkg/apis/sources/v1beta2"
)

type CronJobRunner interface {
	Start(stopCh <-chan struct{})
	Stop()
	AddSchedule(source *v1beta2.PingSource) cron.EntryID
	RemoveSchedule(id cron.EntryID)
}

type cronJobsRunner struct {
	// The cron job runner
	cron cron.Cron

	// client sends cloudevents.
	Client cloudevents.Client

	// Where to send logs
	Logger *zap.SugaredLogger

	// kubeClient for sending k8s events
	kubeClient kubernetes.Interface
}

const (
	resourceGroup = "pingsources.sources.knative.dev"
)

func NewCronJobsRunner(ceClient cloudevents.Client, kubeClient kubernetes.Interface, logger *zap.SugaredLogger, opts ...cron.Option) *cronJobsRunner {
	return &cronJobsRunner{
		cron:       *cron.New(opts...),
		Client:     ceClient,
		Logger:     logger,
		kubeClient: kubeClient,
	}
}

func (a *cronJobsRunner) AddSchedule(source *v1beta2.PingSource) cron.EntryID {
	event, err := makeEvent(source)
	if err != nil {
		a.Logger.Error("failed to makeEvent: ", zap.Error(err))
	}

	ctx := context.Background()
	ctx = cloudevents.ContextWithTarget(ctx, source.Status.SinkURI.String())

	var kubeEventSink record.EventSink = &typedcorev1.EventSinkImpl{Interface: a.kubeClient.CoreV1().Events(source.Namespace)}
	ctx = crstatusevent.ContextWithCRStatus(ctx, &kubeEventSink, "ping-source-mt-adapter", source, a.Logger.Infof)

	// Simple retry configuration to be less than 1mn.
	// We might want to retry more times for less-frequent schedule.
	ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, 50*time.Millisecond, 5)

	metricTag := &kncloudevents.MetricTag{
		Namespace:     source.Namespace,
		Name:          source.Name,
		ResourceGroup: resourceGroup,
	}

	ctx = kncloudevents.ContextWithMetricTag(ctx, metricTag)
	id, _ := a.cron.AddFunc(source.Spec.Schedule, a.cronTick(ctx, event))
	return id
}

func (a *cronJobsRunner) RemoveSchedule(id cron.EntryID) {
	a.cron.Remove(id)
}

func (a *cronJobsRunner) Start(stopCh <-chan struct{}) {
	a.cron.Start()
	<-stopCh
}

func (a *cronJobsRunner) Stop() {
	ctx := a.cron.Stop() // no more ticks
	if ctx != nil {
		// Wait for all jobs to be done.
		<-ctx.Done()
	}
}

func (a *cronJobsRunner) cronTick(ctx context.Context, event cloudevents.Event) func() {
	return func() {
		event := event.Clone()
		event.SetID(uuid.New().String()) // provide an ID here so we can track it with logging
		defer a.Logger.Debug("Finished sending cloudevent id: ", event.ID())
		target := cecontext.TargetFrom(ctx).String()
		source := event.Context.GetSource()

		// Provide a delay so not all ping fired instantaneously distribute load on resources.
		time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond) //nolint:gosec // Cryptographic randomness not necessary here.

		a.Logger.Debugf("sending cloudevent id: %s, source: %s, target: %s", event.ID(), source, target)

		if result := a.Client.Send(ctx, event); !cloudevents.IsACK(result) {
			// Exhausted number of retries. Event is lost.
			a.Logger.Error("failed to send cloudevent result: ", zap.Any("result", result),
				zap.String("source", source), zap.String("target", target), zap.String("id", event.ID()))
		}
	}
}

func makeEvent(source *v1beta2.PingSource) (cloudevents.Event, error) {
	event := cloudevents.NewEvent()
	event.SetType(v1beta2.PingSourceEventType)
	event.SetSource(v1beta2.PingSourceSource(source.Namespace, source.Name))
	if source.Spec.CloudEventOverrides != nil && source.Spec.CloudEventOverrides.Extensions != nil {
		for key, override := range source.Spec.CloudEventOverrides.Extensions {
			event.SetExtension(key, override)
		}
	}

	// Set event data, at most one of data and dataBase64 exists.
	// 1. If dataBase64 exists, then it's binary data, set event.DataEncoded to []byte(dataBase64)
	// 2. If data exists, then it's not binary data
	//  a. If contentType is not `application/json`, set event.DataEncoded to []byte(data)
	//  b. If contentType is `application/json`, unmarshal it into an interface, event.DataEncoded will be json.Marshal(interface),
	//    this is to be compatible with the existing v1beta1 PingSource -> CloudEvent conversion logic, to make sure
	//    that `data` is populated in the cloudevent json format instead of `data_base64`, and not breaking subscribers
	//    that do not leverage cloudevents sdk.
	var data interface{}
	if source.Spec.DataBase64 != "" {
		data = []byte(source.Spec.DataBase64)
	} else if source.Spec.Data != "" {
		switch source.Spec.ContentType {
		case cloudevents.ApplicationJSON:
			// unmarshal the body into an interface, JSON validation is done in pingsource_validation
			// ignoring the error returned by json.Unmarshal here.
			var objmap map[string]*json.RawMessage
			if err := json.Unmarshal([]byte(source.Spec.Data), &objmap); err != nil {
				return event, fmt.Errorf("error unmarshalling source.Spec.Data: %v, err: %v", source.Spec.Data, err)
			}
			data = objmap
		default:
			data = []byte(source.Spec.Data)
		}
	}

	if data != nil {
		if err := event.SetData(source.Spec.ContentType, data); err != nil {
			return event, fmt.Errorf("error when SetData(%v, %v), err: %v", source.Spec.ContentType, data, err)
		}
	}

	return event, nil
}
