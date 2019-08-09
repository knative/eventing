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
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/adapter/apiserver/events"
)

type resource struct {
	ce     cloudevents.Client
	source string
	logger *zap.SugaredLogger
}

var _ cache.Store = (*resource)(nil)

func (a *resource) Add(obj interface{}) error {
	event, err := events.MakeAddEvent(a.source, obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}

	if _, err := a.ce.Send(context.Background(), *event); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
		return err
	}

	return nil
}

func (a *resource) Update(obj interface{}) error {
	event, err := events.MakeUpdateEvent(a.source, obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}

	if _, err := a.ce.Send(context.Background(), *event); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
		return err
	}

	return nil
}

func (a *resource) Delete(obj interface{}) error {
	event, err := events.MakeDeleteEvent(a.source, obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}

	if _, err := a.ce.Send(context.Background(), *event); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
		return err
	}

	return nil
}

func (a *resource) addControllerWatch(gvr schema.GroupVersionResource) {
	// not supported for resource.
	a.logger.Warn("ignored controller watch request on gvr.", zap.String("gvr", gvr.String()))
}

// Stub cache.Store impl

// Implements cache.Store
func (a *resource) List() []interface{} {
	return nil
}

// Implements cache.Store
func (a *resource) ListKeys() []string {
	return nil
}

// Implements cache.Store
func (a *resource) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Implements cache.Store
func (a *resource) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Implements cache.Store
func (a *resource) Replace([]interface{}, string) error {
	return nil
}

// Implements cache.Store
func (a *resource) Resync() error {
	return nil
}
