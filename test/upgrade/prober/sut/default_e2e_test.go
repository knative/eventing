// +build e2e

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

package sut_test

import (
	"context"
	"fmt"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/upgrade/prober/sut"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestNewDefaultE2E(t *testing.T) {
	client := testlib.Setup(t, false)
	defer testlib.TearDown(client)
	s := sut.NewDefault(client.Namespace)
	ctx := sut.Context{
		Ctx:    context.Background(),
		Log:    log(t),
		Client: client,
	}
	// create event logger pod and service as the subscriber
	eis, pod := recordevents.StartEventRecordOrFail(ctx.Ctx, client, "record")
	url, err := s.Deploy(ctx, duckv1.Destination{
		Ref: &duckv1.KReference{
			Kind:       pod.Kind,
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			APIVersion: pod.APIVersion,
		},
	})
	defer func() {
		err := s.Teardown(ctx)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	assert.NotEmpty(t, url)
	ceClient, err := cloudevents.NewClientHTTP()
	assert.NoError(t, err)
	id := uuid.NewString()
	event := cloudevents.NewEvent()
	event.SetID(id)
	err = ceClient.Send(ctx.Ctx, event)
	assert.NoError(t, err)
	eis.AssertExact(1, hasID(id))
}

func hasID(id string) recordevents.EventInfoMatcher {
	return func(info recordevents.EventInfo) error {
		if id != info.Event.ID() {
			return fmt.Errorf(
				"event ID don't match. Expected: %#v, Actual: %#v",
				id, info.Event.ID(),
			)
		}
		return nil
	}
}

func log(t *testing.T) *zap.SugaredLogger {
	l, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err)
	}
	return l.Sugar()
}
