/*
Copyright 2019 The Knative Authors

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

package apiserver

import (
	"context"
	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/knative/eventing/pkg/adapter/apiserver/events"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type resource struct {
	ce     cloudevents.Client
	source string
	logger *zap.SugaredLogger
}

func (a *resource) addEvent(obj interface{}) {
	event, err := events.MakeAddEvent(a.source, obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return
	}

	if _, err := a.ce.Send(context.Background(), *event); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
	}
}

func (a *resource) updateEvent(oldObj, newObj interface{}) {
	event, err := events.MakeUpdateEvent(a.source, oldObj, newObj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return
	}

	if _, err := a.ce.Send(context.Background(), *event); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
	}
}

func (a *resource) deleteEvent(obj interface{}) {
	event, err := events.MakeDeleteEvent(a.source, obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return
	}

	if _, err := a.ce.Send(context.Background(), *event); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
	}
}

func (a *resource) addControllerWatch(gvr schema.GroupVersionResource) {
	// not supported for resource.
	a.logger.Warn("ignored controller watch request on gvr.", zap.String("gvr", gvr.String()))
}
