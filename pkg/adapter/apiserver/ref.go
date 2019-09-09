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
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter/apiserver/events"
)

type ref struct {
	ce        cloudevents.Client
	source    string
	eventType string
	logger    *zap.SugaredLogger

	controlledGVRs []schema.GroupVersionResource
	reporter       StatsReporter
	namespace      string
	name           string
}

var _ cache.Store = (*ref)(nil)

// TODO: I think asController is not the feature we want. I think we want to be
//  able to set the controller as a filter to the watch. Not emit all owners of
//  the resource. Fix this. It has to be an api change on the CRD.

func (a *ref) asController(obj interface{}) bool {
	if len(a.controlledGVRs) > 0 {
		object := obj.(*unstructured.Unstructured)
		gvk := object.GroupVersionKind()
		// TODO: pass down the resource and the kind so we do not have to guess.
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)
		for _, gvrc := range a.controlledGVRs {
			if reflect.DeepEqual(gvr, gvrc) {
				return true
			}
		}
	}
	return false
}

// Implements cache.Store
func (a *ref) Add(obj interface{}) error {
	event, err := events.MakeAddRefEvent(a.source, a.asController(obj), obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}

	return a.sendEvent(context.Background(), event)
}

// Implements cache.Store
func (a *ref) Update(obj interface{}) error {
	event, err := events.MakeUpdateRefEvent(a.source, a.asController(obj), obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}

	return a.sendEvent(context.Background(), event)
}

// Implements cache.Store
func (a *ref) Delete(obj interface{}) error {
	event, err := events.MakeDeleteRefEvent(a.source, a.asController(obj), obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}

	return a.sendEvent(context.Background(), event)
}

func (a *ref) addControllerWatch(gvr schema.GroupVersionResource) {
	if a.controlledGVRs == nil {
		a.controlledGVRs = []schema.GroupVersionResource{gvr}
		return
	}
	a.controlledGVRs = append(a.controlledGVRs, gvr)
}

func (a *ref) sendEvent(ctx context.Context, event *cloudevents.Event) error {
	reportArgs := &ReportArgs{
		ns:          a.namespace,
		eventSource: event.Source(),
		eventType:   event.Type(),
		name:        a.name,
	}

	rctx, _, err := a.ce.Send(ctx, *event)
	if err != nil {
		a.logger.Info("failed to send a ref based event ", zap.Error(err))
	}
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	a.reporter.ReportEventCount(reportArgs, rtctx.StatusCode)
	return err
}

// Stub cache.Store impl

// Implements cache.Store
func (a *ref) List() []interface{} {
	return nil
}

// Implements cache.Store
func (a *ref) ListKeys() []string {
	return nil
}

// Implements cache.Store
func (a *ref) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Implements cache.Store
func (a *ref) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Implements cache.Store
func (a *ref) Replace([]interface{}, string) error {
	return nil
}

// Implements cache.Store
func (a *ref) Resync() error {
	return nil
}
