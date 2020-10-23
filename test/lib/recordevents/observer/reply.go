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

package observer

import (
	"context"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/test/lib/recordevents"
)

func NoOpReply(_ context.Context, writer http.ResponseWriter, _ recordevents.EventInfo) {
	writer.WriteHeader(http.StatusAccepted)
}

func ReplyTransformerFunc(replyEventType string, replyEventSource string, replyEventData string, replyAppendData string) func(context.Context, http.ResponseWriter, recordevents.EventInfo) {
	return func(ctx context.Context, writer http.ResponseWriter, info recordevents.EventInfo) {
		if info.Error != "" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(info.Error))
			logging.FromContext(ctx).Warn("Conversion error in the event to send back", info.Error)
			return
		}

		if info.Event == nil {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("No event!"))
			logging.FromContext(ctx).Warn("No event to send back")
			return
		}

		outputEvent := info.Event.Clone()

		if replyEventSource != "" {
			logging.FromContext(ctx).Infof("Setting reply event source '%s'", replyEventSource)
			outputEvent.SetSource(replyEventSource)
		}
		if replyEventType != "" {
			logging.FromContext(ctx).Infof("Setting reply event type '%s'", replyEventType)
			outputEvent.SetType(replyEventType)
		}
		if replyEventData != "" {
			logging.FromContext(ctx).Infof("Setting reply event data '%s'", replyAppendData)
			if err := outputEvent.SetData(cloudevents.ApplicationJSON, []byte(replyEventData)); err != nil {
				logging.FromContext(ctx).Warn("Cannot set the event data")
			}
		}
		if replyAppendData != "" {
			var d string
			if outputEvent.Data() == nil {
				d = replyAppendData
			} else {
				if err := info.Event.DataAs(&d); err != nil {
					logging.FromContext(ctx).Warn("Cannot read the event data as text/plain")
				}
				d = d + replyAppendData
			}
			logging.FromContext(ctx).Infof("Setting appended event data '%s'", d)
			if err := outputEvent.SetData(cloudevents.TextPlain, d); err != nil {
				logging.FromContext(ctx).Warn("Cannot set the event data")
			}
		}

		logging.FromContext(ctx).Infow("Replying with", zap.Stringer("event", outputEvent))
		err := cehttp.WriteResponseWriter(ctx, binding.ToMessage(&outputEvent), 200, writer)
		if err != nil {
			logging.FromContext(ctx).Warnw("Error while writing the event as response", zap.Error(err))
		}
	}
}
