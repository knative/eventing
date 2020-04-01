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

package channel

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"knative.dev/eventing/pkg/utils"
)

func TestDispatchMessage(t *testing.T) {
	testCases := map[string]struct {
		sendToDestination         bool
		sendToReply               bool
		hasDeadLetterSink         bool
		eventExtensions           map[string]string
		header                    http.Header
		body                      string
		fakeResponse              *http.Response
		fakeReplyResponse         *http.Response
		fakeDeadLetterResponse    *http.Response
		expectedErr               bool
		expectedDestRequest       *requestValidation
		expectedReplyRequest      *requestValidation
		expectedDeadLetterRequest *requestValidation
	}{
		"destination - only": {
			sendToDestination: true,
			header: map[string][]string{
				// do-not-forward should not get forwarded.
				"do-not-forward": {"header"},
				"x-request-id":   {"id123"},
				"knative-1":      {"knative-1-value"},
				"knative-2":      {"knative-2-value"},
			},
			body: "destination",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedDestRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"ignored-value-header"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
		},
		"destination - only -- error": {
			sendToDestination: true,
			header: map[string][]string{
				// do-not-forward should not get forwarded.
				"do-not-forward": {"header"},
				"x-request-id":   {"id123"},
				"knative-1":      {"knative-1-value"},
				"knative-2":      {"knative-2-value"},
			},
			body: "destination",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedDestRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"ignored-value-header"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedErr: true,
		},
		"reply - only": {
			sendToReply: true,
			header: map[string][]string{
				// do-not-forward should not get forwarded.
				"do-not-forward": {"header"},
				"x-request-id":   {"id123"},
				"knative-1":      {"knative-1-value"},
				"knative-2":      {"knative-2-value"},
			},
			body: "reply",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedReplyRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"ignored-value-header"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"reply"`,
			},
		},
		"reply - only -- error": {
			sendToReply: true,
			header: map[string][]string{
				// do-not-forward should not get forwarded.
				"do-not-forward": {"header"},
				"x-request-id":   {"id123"},
				"knative-1":      {"knative-1-value"},
				"knative-2":      {"knative-2-value"},
			},
			body: "reply",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedReplyRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"ignored-value-header"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"reply"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedErr: true,
		},
		"destination and reply - dest returns bad status code": {
			sendToDestination: true,
			sendToReply:       true,
			header: map[string][]string{
				// do-not-forward should not get forwarded.
				"do-not-forward": {"header"},
				"x-request-id":   {"id123"},
				"knative-1":      {"knative-1-value"},
				"knative-2":      {"knative-2-value"},
			},
			body: "destination",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedDestRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"ignored-value-header"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedErr: true,
		},
		"destination and reply - dest returns empty body": {
			sendToDestination: true,
			sendToReply:       true,
			header: map[string][]string{
				// do-not-forward should not get forwarded.
				"do-not-forward": {"header"},
				"x-request-id":   {"id123"},
				"knative-1":      {"knative-1-value"},
				"knative-2":      {"knative-2-value"},
			},
			body: "destination",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedDestRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"ignored-value-header"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: map[string][]string{
					"do-not-passthrough": {"no"},
					"x-request-id":       {"altered-id"},
					"knative-1":          {"new-knative-1-value"},
					"ce-abc":             {`"new-ce-abc-value"`},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			},
		},
		"destination and reply": {
			sendToDestination: true,
			sendToReply:       true,
			header: map[string][]string{
				// do-not-forward should not get forwarded.
				"do-not-forward": {"header"},
				"x-request-id":   {"id123"},
				"knative-1":      {"knative-1-value"},
				"knative-2":      {"knative-2-value"},
			},
			body: "destination",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedDestRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"2002-10-02T15:00:00Z"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: map[string][]string{
					"do-not-passthrough": {"no"},
					"x-request-id":       {"altered-id"},
					"knative-1":          {"new-knative-1-value"},
					"ce-abc":             {`"new-ce-abc-value"`},
					"ce-id":              {"ignored-value-header"},
					"ce-time":            {"2002-10-02T15:00:00Z"},
					"ce-source":          {testCeSource},
					"ce-type":            {testCeType},
					"ce-specversion":     {cloudevents.VersionV1},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedReplyRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"altered-id"},
					"knative-1":      {"new-knative-1-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"new-ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"2002-10-02T15:00:00Z"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: "destination-response",
			},
		},
		"invalid destination and delivery option": {
			sendToDestination: true,
			hasDeadLetterSink: true,
			header: map[string][]string{
				// do-not-forward should not get forwarded.
				"do-not-forward": {"header"},
				"x-request-id":   {"id123"},
				"knative-1":      {"knative-1-value"},
				"knative-2":      {"knative-2-value"},
			},
			body: "destination",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedDestRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"2002-10-02T15:00:00Z"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			expectedDeadLetterRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"2002-10-02T15:00:00Z"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			fakeDeadLetterResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: map[string][]string{
					"do-not-passthrough": {"no"},
					"x-request-id":       {"altered-id"},
					"knative-1":          {"new-knative-1-value"},
					"ce-abc":             {`"new-ce-abc-value"`},
					"ce-id":              {"ignored-value-header"},
					"ce-time":            {"2002-10-02T15:00:00Z"},
					"ce-source":          {testCeSource},
					"ce-type":            {testCeType},
					"ce-specversion":     {cloudevents.VersionV1},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("deadlettersink-response")),
			},
		},
		"destination and invalid reply and delivery option": {
			sendToDestination: true,
			sendToReply:       true,
			hasDeadLetterSink: true,
			header: map[string][]string{
				// do-not-forward should not get forwarded.
				"do-not-forward": {"header"},
				"x-request-id":   {"id123"},
				"knative-1":      {"knative-1-value"},
				"knative-2":      {"knative-2-value"},
			},
			body: "destination",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedDestRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"knative-2":      {"knative-2-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"2002-10-02T15:00:00Z"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			expectedReplyRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"altered-id"},
					"knative-1":      {"new-knative-1-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"new-ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"2002-10-02T15:00:00Z"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: "destination-response",
			},
			expectedDeadLetterRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":   {"altered-id"},
					"knative-1":      {"new-knative-1-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"2002-10-02T15:00:00Z"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: map[string][]string{
					"do-not-passthrough": {"no"},
					"x-request-id":       {"altered-id"},
					"knative-1":          {"new-knative-1-value"},
					"ce-abc":             {`"new-ce-abc-value"`},
					"ce-id":              {"ignored-value-header"},
					"ce-time":            {"2002-10-02T15:00:00Z"},
					"ce-source":          {testCeSource},
					"ce-type":            {testCeType},
					"ce-specversion":     {cloudevents.VersionV1},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			fakeReplyResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       ioutil.NopCloser(bytes.NewBufferString("reply-response")),
			},
			fakeDeadLetterResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: map[string][]string{
					"do-not-passthrough": {"no"},
					"x-request-id":       {"altered-id"},
					"knative-1":          {"new-knative-1-value"},
					"ce-abc":             {`"new-ce-abc-value"`},
					"ce-id":              {"ignored-value-header"},
					"ce-time":            {"2002-10-02T15:00:00Z"},
					"ce-source":          {testCeSource},
					"ce-type":            {testCeType},
					"ce-specversion":     {cloudevents.VersionV1},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("deadlettersink-response")),
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			destHandler := &fakeHandler{
				t:        t,
				response: tc.fakeResponse,
				requests: make([]requestValidation, 0),
			}
			destServer := httptest.NewServer(destHandler)
			defer destServer.Close()

			replyHandler := &fakeHandler{
				t:        t,
				response: tc.fakeResponse,
				requests: make([]requestValidation, 0),
			}
			replyServer := httptest.NewServer(replyHandler)
			defer replyServer.Close()
			if tc.fakeReplyResponse != nil {
				replyHandler.response = tc.fakeReplyResponse
			}

			var deadLetterSinkHandler *fakeHandler
			var deadLetterSinkServer *httptest.Server
			var deadLetterSink *url.URL
			if tc.hasDeadLetterSink {
				deadLetterSinkHandler = &fakeHandler{
					t:        t,
					response: tc.fakeDeadLetterResponse,
					requests: make([]requestValidation, 0),
				}
				deadLetterSinkServer = httptest.NewServer(deadLetterSinkHandler)
				defer deadLetterSinkServer.Close()

				deadLetterSink = getOnlyDomainURL(t, true, deadLetterSinkServer.URL)
			}

			event := cloudevents.NewEvent(cloudevents.VersionV1)
			event.SetID(uuid.New().String())
			event.SetType("testtype")
			event.SetSource("testsource")
			for n, v := range tc.eventExtensions {
				event.SetExtension(n, v)
			}
			event.SetData(cloudevents.ApplicationJSON, tc.body)

			ctx := context.Background()

			md := NewMessageDispatcher(zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())))

			destination := getOnlyDomainURL(t, tc.sendToDestination, destServer.URL)
			reply := getOnlyDomainURL(t, tc.sendToReply, replyServer.URL)

			// We need to do message -> event -> message to emulate the same transformers the event receiver would do
			message := binding.ToMessage(&event)
			var err error
			ev, err := binding.ToEvent(ctx, message, binding.TransformerFactories{transformer.AddTimeNow})
			if err != nil {
				t.Fatal(err)
			}
			message = binding.ToMessage(ev)
			finishInvoked := 0
			message = binding.WithFinish(message, func(err error) {
				finishInvoked++
			})

			err = md.DispatchMessage(ctx, message, utils.PassThroughHeaders(tc.header), destination, reply, deadLetterSink)

			if tc.expectedErr != (err != nil) {
				t.Errorf("Unexpected error from DispatchMessage. Expected %v. Actual: %v", tc.expectedErr, err)
			}
			if finishInvoked != 1 {
				t.Errorf("Finish should be invoked exactly one time. Actual: %d", finishInvoked)
			}
			if tc.expectedDestRequest != nil {
				rv := destHandler.popRequest(t)
				assertEquality(t, destServer.URL, *tc.expectedDestRequest, rv)
			}
			if tc.expectedReplyRequest != nil {
				rv := replyHandler.popRequest(t)
				assertEquality(t, replyServer.URL, *tc.expectedReplyRequest, rv)
			}
			if tc.expectedDeadLetterRequest != nil {
				rv := deadLetterSinkHandler.popRequest(t)
				assertEquality(t, deadLetterSinkServer.URL, *tc.expectedDeadLetterRequest, rv)
			}
			if len(destHandler.requests) != 0 {
				t.Errorf("Unexpected destination requests: %+v", destHandler.requests)
			}
			if len(replyHandler.requests) != 0 {
				t.Errorf("Unexpected reply requests: %+v", replyHandler.requests)
			}
			if deadLetterSinkHandler != nil && len(deadLetterSinkHandler.requests) != 0 {
				t.Errorf("Unexpected dead letter sink requests: %+v", deadLetterSinkHandler.requests)
			}
		})
	}
}

func getOnlyDomainURL(t *testing.T, shouldSend bool, serverURL string) *url.URL {
	if shouldSend {
		server, err := url.Parse(serverURL)
		if err != nil {
			t.Errorf("Bad serverURL: %q", serverURL)
		}
		return &url.URL{
			Host: server.Host,
		}
	}
	return nil
}
