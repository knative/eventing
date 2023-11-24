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
	"github.com/google/uuid"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/adapter/apiserver/events"
)

type resourceDelegate struct {
	ce                  cloudevents.Client
	source              string
	ref                 bool
	apiServerSourceName string

	logger *zap.SugaredLogger
}

var _ cache.Store = (*resourceDelegate)(nil)

func (a *resourceDelegate) Add(obj interface{}) error {
	ctx, event, err := events.MakeAddEvent(a.source, a.apiServerSourceName, obj, a.ref)
	if err != nil {
		a.logger.Infow("event creation failed", zap.Error(err))
		return err
	}
	a.sendCloudEvent(ctx, event)
	return nil
}

func (a *resourceDelegate) Update(obj interface{}) error {
	ctx, event, err := events.MakeUpdateEvent(a.source, a.apiServerSourceName, obj, a.ref)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}
	a.sendCloudEvent(ctx, event)
	return nil
}

func (a *resourceDelegate) Delete(obj interface{}) error {
	ctx, event, err := events.MakeDeleteEvent(a.source, a.apiServerSourceName, obj, a.ref)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return err
	}
	a.sendCloudEvent(ctx, event)
	return nil
}

// sendCloudEvent sends a cloudevent everytime k8s api event is created, updated or deleted.
func (a *resourceDelegate) sendCloudEvent(ctx context.Context, event cloudevents.Event) {
	event.SetID(uuid.New().String()) // provide an ID here so we can track it with logging
	defer a.logger.Debug("Finished sending cloudevent id: ", event.ID())
	source := event.Context.GetSource()
	subject := event.Context.GetSubject()
	a.logger.Debugf("sending cloudevent id: %s, source: %s, subject: %s", event.ID(), source, subject)

	// Decide whether to request the JWT token or not
	// Condition: if the sink has audience or not
	// ?? Question: where can we get the sink audience? As we don't specify the destination in the cloudevent

	// If the sink has audience, then we need to request the JWT token
	// In order to request the JWT token, we need to get the service account name and namespace from the source
	// And also need to pass in OIDC token provider
	// ?? Question again: where can we get the sink audience? And how to pass in OIDC token provider?

	// If the sink doesn't have audience, then we don't need to request the JWT token

	// If the sink has audience, and we have the JWT token, then we need to add the JWT token to the cloudevent
	// Easy to do this, just add the JWT token as the bearer auth header to the cloudevent header

	// Discovery:
	// ReceiveAdapter -> ResourceDelegate -> MakeAddEvent -> MakeEvent -> MakeCloudEvent -> SendCloudEvent
	// Receive adapter is the entry point of the adapter, it receives the k8s api event
	// ResourceDelegate is the cache.Store, it receives the k8s api event from the receive adapter

	//ApiServerSource will listen to the k8s api event, and then send the cloudevent to the sink when the k8s api event is created, updated or deleted

	// Prepare the headers
	//headers := http.HeaderFrom(ctx)
	//jwt := auth.GetJWT(ctx)
	//headers.Set("Authentication", fmt.Print("Bearer %s", jwt))

	if result := a.ce.Send(ctx, event); !cloudevents.IsACK(result) {
		a.logger.Errorw("failed to send cloudevent", zap.Error(result), zap.String("source", source),
			zap.String("subject", subject), zap.String("id", event.ID()))
	} else {
		a.logger.Debugf("cloudevent sent id: %s, source: %s, subject: %s", event.ID(), source, subject)
	}
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
