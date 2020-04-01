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

package apiserver

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/adapter/apiserver/events"
)

type resourceDelegate struct {
	ce     cloudevents.Client
	source string
	ref    bool

	logger *zap.SugaredLogger
}

var _ cache.Store = (*resourceDelegate)(nil)

func (a *resourceDelegate) Add(obj interface{}) error {
	event, err := events.MakeAddEvent(a.source, obj, a.ref)
	if err != nil {
		a.logger.Infow("event creation failed", zap.Error(err))
		return err
	}

	if result := a.ce.Send(context.Background(), event); !cloudevents.IsACK(result) {
		a.logger.Errorw("failed to send event", zap.Error(result))
	}
	return nil
}

func (a *resourceDelegate) Update(obj interface{}) error {
	event, err := events.MakeUpdateEvent(a.source, obj, a.ref)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}

	if result := a.ce.Send(context.Background(), event); !cloudevents.IsACK(result) {
		a.logger.Error("failed to send event", zap.Error(result))
	}
	return nil
}

func (a *resourceDelegate) Delete(obj interface{}) error {
	event, err := events.MakeDeleteEvent(a.source, obj, a.ref)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}

	if result := a.ce.Send(context.Background(), event); !cloudevents.IsACK(result) {
		a.logger.Error("failed to send event", zap.Error(result))
	}
	return nil
}

// Stub cache.Store impl

// Implements cache.Store
func (a *resourceDelegate) List() []interface{} {
	return nil
}

// Implements cache.Store
func (a *resourceDelegate) ListKeys() []string {
	return nil
}

// Implements cache.Store
func (a *resourceDelegate) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Implements cache.Store
func (a *resourceDelegate) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Implements cache.Store
func (a *resourceDelegate) Replace([]interface{}, string) error {
	return nil
}

// Implements cache.Store
func (a *resourceDelegate) Resync() error {
	return nil
}
