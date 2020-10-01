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

package crstatusevent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	v1beta1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

func logF(format string, a ...interface{}) {

}

var src = &v1alpha2.ApiServerSource{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "v1alpha2",
		Kind:       "ApiServerSource",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "nma",
		Namespace: "source-namespace",
		UID:       "1234",
	},
	Spec: v1alpha2.ApiServerSourceSpec{
		Resources: []v1alpha2.APIVersionKindSelector{{
			APIVersion: "",
			Kind:       "Namespace",
		}, {
			APIVersion: "batch/v1",
			Kind:       "Job",
		}, {
			APIVersion: "",
			Kind:       "Pod",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test-key1": "test-value1"},
			},
		}},
		ResourceOwner: &v1alpha2.APIVersionKind{
			APIVersion: "custom/v1",
			Kind:       "Parent",
		},
		EventMode:          "Resource",
		ServiceAccountName: "source-svc-acct",
	},
}
var mutex = &sync.Mutex{}
var recordTestSinkResults map[string]*v1beta1.Event = make(map[string]*v1beta1.Event)

type fakeSink struct {
	record.EventSink
	Name string
}

func (f fakeSink) Create(event *v1beta1.Event) (*v1beta1.Event, error) {
	mutex.Lock()
	defer mutex.Unlock()
	recordTestSinkResults[f.Name] = event
	return event, nil
}

func TestReportCRStatusEvent(t *testing.T) {
	type args struct {
		fakesink    record.EventSink
		result      protocol.Result
		enabled     string
		wantType    string
		wantReason  string
		wantMessage string
	}
	tests := []struct {
		name string
		args args
	}{{
		name: "TestReportCRStatusEvent500",
		args: args{
			fakesink:    fakeSink{Name: "TestReportCRStatusEvent500"},
			result:      http.NewResult(500, "" /*noargs*/),
			enabled:     "true",
			wantType:    "Warning",
			wantReason:  "SinkSendFailed",
			wantMessage: "500 Error sending cloud event to sink.",
		},
	}, {
		name: "TestReportCRStatusEvent200",
		args: args{
			fakesink: fakeSink{Name: "TestReportCRStatusEvent200"},
			enabled:  "true",
			result:   http.NewResult(200, ""),
		},
	}, {
		name: "TestReportCRStatusEvent500Disabled",
		args: args{
			fakesink: fakeSink{Name: "TestReportCRStatusEvent500"},
			result:   http.NewResult(500, ""),
			enabled:  "false",
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			defer ctx.Done()
			ctx = ContextWithCRStatus(ctx, &tt.args.fakesink, "mycomponent", src, logF)

			crStatusEventClient := NewCRStatusEventClient(map[string]string{"sink-event-error-reporting.enable": tt.args.enabled})

			crStatusEventClient.ReportCRStatusEvent(ctx, tt.args.result)

			time.Sleep(time.Millisecond * 500)
			mutex.Lock()
			defer mutex.Unlock()
			event := recordTestSinkResults[tt.name]

			if tt.args.wantType == "" && event != nil {
				t.Error("Expected no event but got one: ", event)
			} else if event == nil {
				return // all good return.
			}
			if tt.args.wantType != "" && event == nil {
				t.Error("Wanted event got nil")
			}
			if tt.args.wantType != event.Type {
				t.Errorf("Wanted warning got %s", tt.args.wantType)
			}
			if event.Reason != tt.args.wantReason {
				t.Errorf("Reason = %q; want %q", event.Reason, tt.args.wantReason)
			}
			if event.Message != tt.args.wantMessage {
				t.Errorf("Message = %q; want: %q", event.Message, tt.args.wantMessage)
			}

		})
	}
}
