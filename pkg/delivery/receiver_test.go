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
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/elafros/eventing/pkg/apis/bind/v1alpha1"
	"github.com/elafros/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/elafros/eventing/pkg/client/informers/externalversions"
	"github.com/elafros/eventing/pkg/delivery"
	"github.com/elafros/eventing/pkg/delivery/queue"
	"github.com/elafros/eventing/pkg/event"
)

var (
	demoBind = &v1alpha1.Bind{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Bind",
			APIVersion: "eventing.elafros.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: v1alpha1.BindSpec{
			Trigger: v1alpha1.EventTrigger{
				EventType: "org.example.object.create",
				Service:   "example.org",
				Resource:  "{abc}/123",
			},
			Action: v1alpha1.BindAction{
				RouteName: "vaikas.fix.this",
			},
		},
	}
)

func TestReceiverHttpErrors(t *testing.T) {
	tests := []struct {
		description    string
		method         string
		path           string
		headers        http.Header
		body           interface{}
		expectedStatus int
	}{
		{
			description:    "bad method",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			description:    "bad path",
			method:         http.MethodPost,
			path:           "/abc/123",
			expectedStatus: http.StatusNotFound,
		},
		{
			description: "bad event",
			method:      http.MethodPost,
			path:        "/v1alpha1/namespaces/default/flows/demo:sendEvent",
			// no headers, so we're missing all default fields
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			client := fake.NewSimpleClientset(demoBind)
			factory := informers.NewSharedInformerFactory(client, 0).Eventing().V1alpha1().Binds()
			factory.Informer().GetIndexer().Add(demoBind)
			lister := factory.Lister()

			q := queue.NewInMemoryQueue(1)
			r := delivery.NewReceiver(lister, q)
			s := httptest.NewServer(r)
			defer s.Close()

			url, err := url.Parse(s.URL + test.path)
			if err != nil {
				t.Fatalf("Unexpected error crafting test URL: %s", err)
			}
			res, err := s.Client().Do(&http.Request{
				Method: test.method,
				URL:    url,
			})
			if err != nil {
				t.Fatalf("Failed to reach test server: %s", err)
			}
			if res.StatusCode != test.expectedStatus {
				t.Fatalf("Wrong status; expected=%d got=%d", test.expectedStatus, res.StatusCode)
			}
		})
	}
}

func TestEnqueueEvent(t *testing.T) {
	client := fake.NewSimpleClientset(demoBind)
	factory := informers.NewSharedInformerFactory(client, 0).Eventing().V1alpha1().Binds()
	factory.Informer().GetIndexer().Add(demoBind)
	lister := factory.Lister()

	q := queue.NewInMemoryQueue(1)
	r := delivery.NewReceiver(lister, q)
	s := httptest.NewServer(r)
	defer s.Close()

	data := map[string]interface{}{"hello": "world"}
	context := &event.Context{
		CloudEventsVersion: "0.0.1-rc1",
		EventID:            "123abc",
		EventTime:          time.Now().UTC(),
		EventType:          "org.example.object.create",
		Source:             "https://example.org/examples/example1",
	}

	path := s.URL + "/v1alpha1/namespaces/default/flows/demo:sendEvent"
	req, err := event.NewRequest(path, data, *context)
	if err != nil {
		t.Fatalf("Failed to create request: %s", err)
	}

	res, err := s.Client().Do(req)
	if err != nil {
		t.Fatalf("Unexpected error calling sendEvent: %s", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("SendEvent request %+v failed with response %+v", req, res)
	}

	if q.Length() != 1 {
		t.Fatalf("Expected one queued event; got %d", q.Length())
	}
	event, ok := q.Pull(nil)
	if !ok {
		t.Fatal("Failed to pull queued event")
	}
	if !reflect.DeepEqual(context, event.Context) {
		t.Fatalf("Event context was not marshalled correctly; expected=%+v got=%+v", context, event.Context)
	}
	if !reflect.DeepEqual(data, event.Data) {
		t.Fatalf("Event data was not marshalled correctly; expected=%+v got=%+v", data, event.Data)
	}
}
