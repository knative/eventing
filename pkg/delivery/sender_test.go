/*
Copyright 2018 Google, Inc. All rights reserved.

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

package delivery_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/elafros/eventing/pkg/apis/bind/v1alpha1"
	"github.com/elafros/eventing/pkg/delivery"
	"github.com/elafros/eventing/pkg/delivery/action"
	"github.com/elafros/eventing/pkg/delivery/queue"
	"github.com/elafros/eventing/pkg/event"
)

func TestSendEvent(t *testing.T) {
	actionInvoked := false
	expectedData := map[string]interface{}{
		"hello": "world",
	}
	expectedContext := &event.Context{
		EventID: "123",
	}
	actionName := "projects/demo/region/us-central1/function/test"
	actionType := "functions.googleapis.com"
	callback := func(name string, data interface{}, context *event.Context) (interface{}, error) {
		if actionInvoked {
			t.Fatal("Action invoked twice")
		}
		actionInvoked = true
		if name != actionName {
			t.Fatalf("Action invoked with wrong name. expected=%s got=%s", actionName, name)
		}
		if !reflect.DeepEqual(expectedData, data) {
			t.Fatalf("Action invoked with wrong data. expected=%v got=%v", expectedData, data)
		}
		if !reflect.DeepEqual(expectedContext, context) {
			t.Fatalf("Action invoked with wrong context. expected=%v got=%v", expectedContext, context)
		}

		return nil, nil
	}

	q := queue.NewInMemoryQueue(1)
	sender := delivery.NewSender(q, map[string]action.Action{
		actionType: action.ActionFunc(callback),
	})

	q.Push(queue.QueuedEvent{
		Context: expectedContext,
		Data:    expectedData,
		Action: v1alpha1.BindAction{
			Processor: actionType,
			Name:      actionName,
		},
	})

	if err := sender.RunOnce(nil); err != nil {
		t.Fatalf("RunOnce() failed with err: %s", err)
	}
	if !actionInvoked {
		t.Fatal("Did not invoke action")
	}
}

func TestShutdown(t *testing.T) {
	q := queue.NewInMemoryQueue(0)
	sender := delivery.NewSender(q, nil)
	stopCh := make(chan struct{})
	var wait sync.WaitGroup
	done := false
	wait.Add(1)
	go func() {
		sender.Run(20, stopCh)
		done = true
		wait.Done()
	}()
	if done {
		t.Fatalf("Sender stops automatically")
	}
	close(stopCh)
	wait.Wait()
	if !done {
		t.Fatalf("Should be done")
	}
}
