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
	"reflect"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"k8s.io/apiserver/pkg/storage/names"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/upgrade/prober/sut"
	watholaevent "knative.dev/eventing/test/upgrade/prober/wathola/event"
	watholasender "knative.dev/eventing/test/upgrade/prober/wathola/sender"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/signals"
	pkgTest "knative.dev/pkg/test"
)

// TestBrokerAndTriggers isn't executed by regular E2E tests as it's actually
// executed as part of the upgrade tests. This test here is to make it easy to
// create other SUTs for Eventing and quickly verify them act properly.
func TestBrokerAndTriggers(t *testing.T) {
	ctx := signals.NewContext()
	client := testlib.Setup(t, false)
	defer testlib.TearDown(client)
	s := sut.NewBrokerAndTriggers()
	sutCtx := sut.Context{
		Ctx:    ctx,
		Log:    log(t),
		Client: client,
	}
	// create event logger pod and service as the subscriber
	receiverName := "receiver"
	eis, pod := recordevents.StartEventRecordOrFail(ctx, client, receiverName)

	ref := pkgTest.CoreV1ObjectReference(
		resources.ServiceKind, resources.CoreAPIVersion, receiverName)
	endpoint := s.Deploy(sutCtx, duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: ref.APIVersion,
			Kind:       ref.Kind,
			Namespace:  pod.Namespace,
			Name:       pod.Name,
		},
	})
	if tr, ok := s.(sut.HasTeardown); ok {
		defer tr.Teardown(sutCtx)
	}
	assert.NotEmpty(t, endpoint)

	client.WaitForAllTestResourcesReadyOrFail(ctx)
	step1 := watholasender.NewCloudEvent(watholaevent.Step{Number: 1},
		watholaevent.StepType)
	step2 := watholasender.NewCloudEvent(watholaevent.Step{Number: 2},
		watholaevent.StepType)
	finished := watholasender.NewCloudEvent(watholaevent.Finished{EventsSent: 2},
		watholaevent.FinishedType)

	sender := testingSender{t: t, ctx: ctx, client: client, url: endpoint.(*apis.URL)}
	sender.send(step1)
	sender.send(step2)
	sender.send(finished)

	eis.AssertAtLeast(2, eventType(watholaevent.StepType))
	eis.AssertAtLeast(1, eventType(watholaevent.FinishedType))
	eis.AssertAtLeast(1, stepEvent(watholaevent.Step{Number: 1}))
	eis.AssertAtLeast(1, stepEvent(watholaevent.Step{Number: 2}))
	eis.AssertAtLeast(1, finishedEvent(watholaevent.Finished{EventsSent: 2}))
}

type testingSender struct {
	t      *testing.T
	ctx    context.Context
	client *testlib.Client
	url    *apis.URL
}

func (s *testingSender) send(event cloudevents.Event) {
	s.t.Helper()
	senderName := names.SimpleNameGenerator.GenerateName("sender-")
	s.client.SendEvent(s.ctx, senderName, s.url.String(), event)
}

func eventType(_type string) recordevents.EventInfoMatcher {
	return func(info recordevents.EventInfo) error {
		actual := info.Event.Type()
		if actual == _type {
			return nil
		}
		return fmt.Errorf("event type don't match. want: '%s', got: '%s'",
			_type, actual)
	}
}

func finishedEvent(expected watholaevent.Finished) recordevents.EventInfoMatcher {
	return func(info recordevents.EventInfo) error {
		ce := info.Event
		if ce.Type() == watholaevent.FinishedType {
			actual := &watholaevent.Finished{}
			err := ce.DataAs(actual)
			if err != nil {
				return err
			}
			if reflect.DeepEqual(actual, expected) {
				return fmt.Errorf(
					"finished event don't match. want: %#v, got: %#v",
					expected, actual,
				)
			}
		}
		return nil
	}
}

func stepEvent(expected watholaevent.Step) recordevents.EventInfoMatcher {
	return func(info recordevents.EventInfo) error {
		ce := info.Event
		if ce.Type() == watholaevent.StepType {
			actual := &watholaevent.Step{}
			err := ce.DataAs(actual)
			if err != nil {
				return err
			}
			if reflect.DeepEqual(actual, expected) {
				return fmt.Errorf(
					"step event don't match. want: %#v, got: %#v",
					expected, actual,
				)
			}
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
