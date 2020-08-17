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
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	kncloudevents "knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/eventing/pkg/utils/cache"
)

type cronJobsRunner struct {
	// The cron job runner
	cron cron.Cron

	// client sends cloudevents.
	Client cloudevents.Client

	// Where to send logs
	Logger *zap.SugaredLogger

	// entryids records created cron jobs with the corresponding config
	entryids map[string]entryIdConfig // key: resource namespace/name

	// kubeClient for sending k8s events
	kubeClient kubernetes.Interface
}

type entryIdConfig struct {
	entryID cron.EntryID
	config  *PingConfig
}

const (
	resourceGroup = "pingsources.sources.knative.dev"
)

func NewCronJobsRunner(ceClient cloudevents.Client, kubeClient kubernetes.Interface, logger *zap.SugaredLogger, opts ...cron.Option) *cronJobsRunner {
	return &cronJobsRunner{
		cron:       *cron.New(opts...),
		Client:     ceClient,
		Logger:     logger,
		entryids:   make(map[string]entryIdConfig),
		kubeClient: kubeClient,
	}
}

func (a *cronJobsRunner) AddSchedule(cfg PingConfig) cron.EntryID {
	event := cloudevents.NewEvent()
	event.SetType(sourcesv1alpha2.PingSourceEventType)
	event.SetSource(sourcesv1alpha2.PingSourceSource(cfg.Namespace, cfg.Name))
	event.SetData(cloudevents.ApplicationJSON, message(cfg.JsonData))
	if cfg.Extensions != nil {
		for key, override := range cfg.Extensions {
			event.SetExtension(key, override)
		}
	}

	ctx := context.Background()
	ctx = cloudevents.ContextWithTarget(ctx, cfg.SinkURI)

	var kubeEventSink record.EventSink = &typedcorev1.EventSinkImpl{Interface: a.kubeClient.CoreV1().Events(cfg.Namespace)}
	ctx = crstatusevent.ContextWithCRStatus(ctx, &kubeEventSink, "ping-source-mt-adapter", &cfg.ObjectReference, a.Logger.Infof)

	// Simple retry configuration to be less than 1mn.
	// We might want to retry more times for less-frequent schedule.
	ctx = cloudevents.ContextWithRetriesExponentialBackoff(ctx, 50*time.Millisecond, 5)

	metricTag := &kncloudevents.MetricTag{
		Namespace:     cfg.Namespace,
		Name:          cfg.Name,
		ResourceGroup: resourceGroup,
	}
	ctx = kncloudevents.ContextWithMetricTag(ctx, metricTag)
	id, _ := a.cron.AddFunc(cfg.Schedule, a.cronTick(ctx, event))
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
		// wait for all jobs to be done.
		<-ctx.Done()
	}
}

func (a *cronJobsRunner) cronTick(ctx context.Context, event cloudevents.Event) func() {
	return func() {
		if result := a.Client.Send(ctx, event); !cloudevents.IsACK(result) {
			// Exhausted number of retries. Event is lost.
			a.Logger.Error("failed to send cloudevent", zap.Any("result", result))
		}
	}
}

type Message struct {
	Body string `json:"body"`
}

func message(body string) interface{} {
	// try to marshal the body into an interface.
	var objmap map[string]*json.RawMessage
	if err := json.Unmarshal([]byte(body), &objmap); err != nil {
		// default to a wrapped message.
		return Message{Body: body}
	}
	return objmap
}

func (a *cronJobsRunner) updateFromConfigMap(cm *corev1.ConfigMap) {
	a.Logger.Info("synchronizing configmap")
	data, ok := cm.Data[cache.ResourcesKey]
	if !ok {
		// Shouldn't happened.
		a.Logger.Warn("missing configmap key", zap.Any("key", cache.ResourcesKey))
		return
	}

	var cfgs PingConfigs
	err := json.Unmarshal([]byte(data), &cfgs)
	if err != nil {
		// Shouldn't happened.
		a.Logger.Warn("cannot unmarshal ping source configuration", zap.Error(err))
		return
	}

	keys := make(map[string]bool)
	for k := range a.entryids {
		keys[k] = true
	}

	for key, cfg := range cfgs {
		cfg.APIVersion = sourcesv1alpha2.SchemeGroupVersion.String()
		cfg.Kind = "PingSource"

		// Is the schedule already cached?
		if entry, ok := a.entryids[key]; ok {
			if !equality.Semantic.DeepEqual(entry.config, &cfg) {
				a.Logger.Info("updating schedule ", zap.String("key", key))

				// Recreate cronjob
				a.RemoveSchedule(entry.entryID)

				a.entryids[key] = entryIdConfig{
					entryID: a.AddSchedule(cfg),
					config:  &cfg,
				}
			} else {
				// cron jon exists and correctly configure. noop.
			}
		} else {
			a.Logger.Info("adding schedule ", zap.String("key", key))
			// Create cronjob
			a.entryids[key] = entryIdConfig{
				entryID: a.AddSchedule(cfg),
				config:  &cfg,
			}
		}

		delete(keys, key)
	}

	for key := range keys {
		if entry, ok := a.entryids[key]; ok {
			a.Logger.Info("deleting schedule", zap.Any("key", key))
			a.RemoveSchedule(entry.entryID)
			delete(a.entryids, key)
		}
	}
}
