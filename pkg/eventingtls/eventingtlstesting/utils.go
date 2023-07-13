/*
Copyright 2023 The Knative Authors

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

package eventingtlstesting

import (
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

// EventChannelHandler is an http.Handler that covert received requests to cloudevent.Event and then
// send them to the given channel.
func EventChannelHandler(requestsChan chan<- cloudevents.Event) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if requestsChan != nil {
			message := cehttp.NewMessageFromHttpRequest(request)
			defer message.Finish(nil)

			event, err := binding.ToEvent(request.Context(), message)
			if err != nil {
				panic("Expected event: " + err.Error())
			}
			requestsChan <- *event
		}
		if request.TLS == nil {
			// It's not on TLS, fail request
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusOK)
	})
}
