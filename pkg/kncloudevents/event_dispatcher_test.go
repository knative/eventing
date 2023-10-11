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

package kncloudevents_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	rectesting "knative.dev/pkg/reconciler/testing"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/utils"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	_ "knative.dev/pkg/system/testing"
)

var (
	// Headers that are added to the response, but we don't want to check in our assertions.
	unimportantHeaders = sets.NewString(
		"accept-encoding",
		"content-length",
		"content-type",
		"user-agent",
		"tracestate",
		"ce-tracestate",
	)

	// Headers that should be present, but their value should not be asserted.
	ignoreValueHeaders = sets.NewString(
		// These are headers added for tracing, they will have random values, so don't bother
		// checking them.
		"traceparent",
		// CloudEvents headers, they will have random values, so don't bother checking them.
		"ce-id",
		"ce-time",
		"ce-traceparent",
	)
)

const (
	testCeSource = "testsource"
	testCeType   = "testtype"
)

func TestSendEvent(t *testing.T) {
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
		lastReceiver              string
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
					"prefer":         {"reply"},
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
			lastReceiver: "destination",
		},
		"destination - nil additional headers": {
			sendToDestination: true,
			body:              "destination",
			eventExtensions: map[string]string{
				"abc": `"ce-abc-value"`,
			},
			expectedDestRequest: &requestValidation{
				Headers: map[string][]string{
					"prefer":         {"reply"},
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
			lastReceiver: "destination",
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
					"prefer":         {"reply"},
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
				Body:       io.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedErr:  true,
			lastReceiver: "destination",
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
			lastReceiver: "reply",
			expectedErr:  true,
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
					"prefer":         {"reply"},
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
				Body:       io.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedErr:  true,
			lastReceiver: "reply",
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
					"prefer":         {"reply"},
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
				Body: io.NopCloser(bytes.NewBufferString("")),
			},
			lastReceiver: "reply",
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
					"prefer":         {"reply"},
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
				Body: io.NopCloser(bytes.NewBufferString("destination-response")),
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
			lastReceiver: "reply",
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
					"prefer":         {"reply"},
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
					"x-request-id":        {"id123"},
					"knative-1":           {"knative-1-value"},
					"knative-2":           {"knative-2-value"},
					"traceparent":         {"ignored-value-header"},
					"ce-abc":              {`"ce-abc-value"`},
					"ce-knativeerrorcode": {strconv.Itoa(http.StatusBadRequest)},
					"ce-knativeerrordata": {base64.StdEncoding.EncodeToString([]byte("destination-response"))},
					"ce-id":               {"ignored-value-header"},
					"ce-time":             {"2002-10-02T15:00:00Z"},
					"ce-source":           {testCeSource},
					"ce-type":             {testCeType},
					"ce-specversion":      {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString("destination-response")),
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
				Body: io.NopCloser(bytes.NewBufferString("deadlettersink-response")),
			},
			lastReceiver: "deadLetter",
		},
		"invalid destination and delivery option - deadletter reply without event": {
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
					"prefer":         {"reply"},
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
					"x-request-id":        {"id123"},
					"knative-1":           {"knative-1-value"},
					"knative-2":           {"knative-2-value"},
					"traceparent":         {"ignored-value-header"},
					"ce-abc":              {`"ce-abc-value"`},
					"ce-id":               {"ignored-value-header"},
					"ce-knativeerrorcode": {strconv.Itoa(http.StatusBadRequest)},
					"ce-knativeerrordata": {base64.StdEncoding.EncodeToString([]byte("destination-response"))},
					"ce-time":             {"2002-10-02T15:00:00Z"},
					"ce-source":           {testCeSource},
					"ce-type":             {testCeType},
					"ce-specversion":      {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString("destination-response")),
			},
			fakeDeadLetterResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(bytes.NewBufferString("deadlettersink-response")),
			},
			lastReceiver: "deadLetter",
		},
		"invalid reply and delivery option - deadletter reply without event": {
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
					"prefer":         {"reply"},
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
				Body: io.NopCloser(bytes.NewBufferString("destination-response")),
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
			fakeReplyResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString("reply-response-body")),
			},
			expectedDeadLetterRequest: &requestValidation{
				Headers: map[string][]string{
					"x-request-id":        {"altered-id"},
					"knative-1":           {"new-knative-1-value"},
					"traceparent":         {"ignored-value-header"},
					"ce-abc":              {`"ce-abc-value"`},
					"ce-id":               {"ignored-value-header"},
					"ce-knativeerrorcode": {strconv.Itoa(http.StatusBadRequest)},
					"ce-knativeerrordata": {base64.StdEncoding.EncodeToString([]byte("reply-response-body"))},
					"ce-time":             {"2002-10-02T15:00:00Z"},
					"ce-source":           {testCeSource},
					"ce-type":             {testCeType},
					"ce-specversion":      {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeDeadLetterResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(bytes.NewBufferString("deadlettersink-response-body")),
			},
			lastReceiver: "deadLetter",
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
					"prefer":         {"reply"},
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
					"x-request-id":        {"altered-id"},
					"knative-1":           {"new-knative-1-value"},
					"traceparent":         {"ignored-value-header"},
					"ce-abc":              {`"ce-abc-value"`},
					"ce-id":               {"ignored-value-header"},
					"ce-knativeerrorcode": {strconv.Itoa(http.StatusBadRequest)},
					"ce-knativeerrordata": {base64.StdEncoding.EncodeToString([]byte("reply-response"))},
					"ce-time":             {"2002-10-02T15:00:00Z"},
					"ce-source":           {testCeSource},
					"ce-type":             {testCeType},
					"ce-specversion":      {cloudevents.VersionV1},
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
				Body: io.NopCloser(bytes.NewBufferString("destination-response")),
			},
			fakeReplyResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString("reply-response")),
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
				Body: io.NopCloser(bytes.NewBufferString("deadlettersink-response")),
			},
			lastReceiver: "deadLetter",
		},
		"invalid characters in failed response body": {
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
					"prefer":         {"reply"},
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
					"x-request-id":        {"id123"},
					"knative-1":           {"knative-1-value"},
					"knative-2":           {"knative-2-value"},
					"traceparent":         {"ignored-value-header"},
					"ce-abc":              {`"ce-abc-value"`},
					"ce-knativeerrorcode": {strconv.Itoa(http.StatusBadRequest)},
					"ce-knativeerrordata": {base64.StdEncoding.EncodeToString([]byte("destination\n multi-line\n response"))},
					"ce-id":               {"ignored-value-header"},
					"ce-time":             {"2002-10-02T15:00:00Z"},
					"ce-source":           {testCeSource},
					"ce-type":             {testCeType},
					"ce-specversion":      {cloudevents.VersionV1},
				},
				Body: `"destination"`,
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString("destination\n multi-line\n response")),
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
				Body: io.NopCloser(bytes.NewBufferString("deadlettersink-response")),
			},
			lastReceiver: "deadLetter",
		},
		"no restriction on message response size": {
			sendToDestination: true,
			sendToReply:       true,
			hasDeadLetterSink: false,
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
					"prefer":         {"reply"},
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
					"x-request-id":   {"id123"},
					"knative-1":      {"knative-1-value"},
					"traceparent":    {"ignored-value-header"},
					"ce-abc":         {`"ce-abc-value"`},
					"ce-id":          {"ignored-value-header"},
					"ce-time":        {"2002-10-02T15:00:00Z"},
					"ce-source":      {testCeSource},
					"ce-type":        {testCeType},
					"ce-specversion": {cloudevents.VersionV1},
				},
				Body: strings.Repeat("a", 2000),
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: map[string][]string{
					"do-not-passthrough": {"no"},
					"x-request-id":       {"id123"},
					"knative-1":          {"knative-1-value"},
					"ce-abc":             {`"ce-abc-value"`},
					"ce-id":              {"ignored-value-header"},
					"ce-time":            {"2002-10-02T15:00:00Z"},
					"ce-source":          {testCeSource},
					"ce-type":            {testCeType},
					"ce-specversion":     {cloudevents.VersionV1},
				},
				Body: io.NopCloser(bytes.NewBufferString(strings.Repeat("a", 2000))),
			},

			lastReceiver: "reply",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx := context.Background()
			ctx, _ = fakekubeclient.With(ctx)
			ctx = injection.WithConfig(ctx, &rest.Config{})

			oidcTokenProvider := auth.NewOIDCTokenProvider(ctx)
			dispatcher := kncloudevents.NewDispatcher(oidcTokenProvider)
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
			var deadLetterSink *duckv1.Addressable
			if tc.hasDeadLetterSink {
				deadLetterSinkHandler = &fakeHandler{
					t:        t,
					response: tc.fakeDeadLetterResponse,
					requests: make([]requestValidation, 0),
				}
				deadLetterSinkServer = httptest.NewServer(deadLetterSinkHandler)
				defer deadLetterSinkServer.Close()

				deadLetterSink = &duckv1.Addressable{
					URL: getOnlyDomainURL(t, true, deadLetterSinkServer.URL),
				}
			}

			event := cloudevents.NewEvent(cloudevents.VersionV1)
			event.SetID(uuid.New().String())
			event.SetType("testtype")
			event.SetSource("testsource")
			for n, v := range tc.eventExtensions {
				event.SetExtension(n, v)
			}
			event.SetData(cloudevents.ApplicationJSON, tc.body)

			ctx = context.Background()

			destination := duckv1.Addressable{
				URL: getOnlyDomainURL(t, tc.sendToDestination, destServer.URL),
			}
			reply := &duckv1.Addressable{
				URL: getOnlyDomainURL(t, tc.sendToReply, replyServer.URL),
			}

			// We need to do message -> event -> message to emulate the same transformers the event receiver would do
			message := binding.ToMessage(&event)
			var err error
			ev, err := binding.ToEvent(ctx, message, binding.Transformers{transformer.AddTimeNow})
			if err != nil {
				t.Fatal(err)
			}
			message = binding.ToMessage(ev)
			finishInvoked := 0
			message = binding.WithFinish(message, func(err error) {
				finishInvoked++
			})

			var headers http.Header = nil
			if tc.header != nil {
				headers = utils.PassThroughHeaders(tc.header)
			}
			info, err := dispatcher.SendMessage(ctx, message, destination,
				kncloudevents.WithReply(reply),
				kncloudevents.WithDeadLetterSink(deadLetterSink),
				kncloudevents.WithHeader(headers))

			if tc.lastReceiver != "" {
				switch tc.lastReceiver {
				case "destination":
					if tc.fakeResponse != nil {
						if tc.fakeResponse.StatusCode != info.ResponseCode {
							t.Errorf("Unexpected response code in DispatchResultInfo. Expected %v. Actual: %v", tc.fakeResponse.StatusCode, info.ResponseCode)
						}
					}
				case "deadLetter":
					if tc.fakeDeadLetterResponse != nil {
						if tc.fakeDeadLetterResponse.StatusCode != info.ResponseCode {
							t.Errorf("Unexpected response code in DispatchResultInfo. Expected %v. Actual: %v", tc.fakeDeadLetterResponse.StatusCode, info.ResponseCode)
						}
					}
				case "reply":
					if tc.fakeReplyResponse != nil {
						if tc.fakeReplyResponse.StatusCode != info.ResponseCode {
							t.Errorf("Unexpected response code in DispatchResultInfo. Expected %v. Actual: %v", tc.fakeReplyResponse.StatusCode, info.ResponseCode)
						}
					}
				}
			}

			if tc.expectedErr != (err != nil) {
				t.Errorf("Unexpected error from DispatchMessage. Expected %v. Actual: %v", tc.expectedErr, err)
			}
			if finishInvoked != 1 {
				t.Error("Finish should be invoked exactly one time. Actual:", finishInvoked)
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
				if tc.sendToReply {
					tc.expectedDeadLetterRequest.Headers.Set("ce-knativeerrordest", replyServer.URL+"/")
				} else if tc.sendToDestination {
					tc.expectedDeadLetterRequest.Headers.Set("ce-knativeerrordest", destServer.URL+"/")
				}
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

func TestDispatchMessageToTLSEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	ctx, _ := rectesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		// give the servers a bit time to fully shutdown to prevent port clashes
		time.Sleep(500 * time.Millisecond)
	}()
	oidcTokenProvider := auth.NewOIDCTokenProvider(ctx)
	dispatcher := kncloudevents.NewDispatcher(oidcTokenProvider)
	eventToSend := test.FullEvent()

	// destination
	destinationEventsChan := make(chan cloudevents.Event, 10)
	destinationReceivedEvents := make([]cloudevents.Event, 0, 10)
	destinationHandler := eventingtlstesting.EventChannelHandler(destinationEventsChan)
	destinationCA := eventingtlstesting.StartServer(ctx, t, 8334, destinationHandler, kncloudevents.WithDrainQuietPeriod(time.Millisecond))
	destination := duckv1.Addressable{
		URL:     apis.HTTPS("localhost:8334"),
		CACerts: &destinationCA,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range destinationEventsChan {
			destinationReceivedEvents = append(destinationReceivedEvents, event)
		}
	}()

	// send event
	message := binding.ToMessage(&eventToSend)
	info, err := dispatcher.SendMessage(ctx, message, destination)
	require.Nil(t, err)
	require.Equal(t, 200, info.ResponseCode)

	// check received events
	close(destinationEventsChan)
	wg.Wait()

	require.Len(t, destinationReceivedEvents, 1)
	require.Equal(t, eventToSend.ID(), destinationReceivedEvents[0].ID())
	require.Equal(t, eventToSend.Data(), destinationReceivedEvents[0].Data())
}

func TestDispatchMessageToTLSEndpointWithReply(t *testing.T) {
	var wg sync.WaitGroup
	ctx, _ := rectesting.SetupFakeContext(t)
	ctxDestination, cancelDestination := context.WithCancel(ctx)
	ctxReply, cancelReply := context.WithCancel(ctx)
	defer func() {
		cancelDestination()
		cancelReply()
		// give the servers a bit time to fully shutdown to prevent port clashes
		time.Sleep(500 * time.Millisecond)
	}()
	oidcTokenProvider := auth.NewOIDCTokenProvider(ctx)
	dispatcher := kncloudevents.NewDispatcher(oidcTokenProvider)

	eventToSend := test.FullEvent()
	eventToReply := test.FullEvent()

	// destination
	destinationHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// reply with the eventToReply event
		w.Header().Add("ce-id", eventToReply.ID())
		w.Header().Add("ce-specversion", eventToReply.SpecVersion())

		w.Write(eventToReply.Data())
	})

	destinationCA := eventingtlstesting.StartServer(ctxDestination, t, 8334, destinationHandler, kncloudevents.WithDrainQuietPeriod(time.Millisecond))
	destination := duckv1.Addressable{
		URL:     apis.HTTPS("localhost:8334"),
		CACerts: &destinationCA,
	}

	// reply
	replyEventChan := make(chan cloudevents.Event, 10)
	replyHandler := eventingtlstesting.EventChannelHandler(replyEventChan)
	replyReceivedEvents := make([]cloudevents.Event, 0, 10)
	replyCA := eventingtlstesting.StartServer(ctxReply, t, 8335, replyHandler, kncloudevents.WithDrainQuietPeriod(time.Millisecond))
	reply := duckv1.Addressable{
		URL:     apis.HTTPS("localhost:8335"),
		CACerts: &replyCA,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range replyEventChan {
			replyReceivedEvents = append(replyReceivedEvents, event)
		}
	}()

	// send event
	message := binding.ToMessage(&eventToSend)
	info, err := dispatcher.SendMessage(ctx, message, destination, kncloudevents.WithReply(&reply))
	require.Nil(t, err)
	require.Equal(t, 200, info.ResponseCode)

	// check received events
	close(replyEventChan)
	wg.Wait()

	require.Len(t, replyReceivedEvents, 1)
	require.Equal(t, eventToReply.ID(), replyReceivedEvents[0].ID())
	require.Equal(t, eventToReply.Data(), replyReceivedEvents[0].Data())
}

func TestDispatchMessageToTLSEndpointWithDeadLetterSink(t *testing.T) {
	var wg sync.WaitGroup
	ctx, _ := rectesting.SetupFakeContext(t)
	ctxDestination, cancelDestination := context.WithCancel(ctx)
	ctxDls, cancelDls := context.WithCancel(ctx)
	defer func() {
		cancelDestination()
		cancelDls()
		// give the servers a bit time to fully shutdown to prevent port clashes
		time.Sleep(500 * time.Millisecond)
	}()
	oidcTokenProvider := auth.NewOIDCTokenProvider(ctx)
	dispatcher := kncloudevents.NewDispatcher(oidcTokenProvider)
	eventToSend := test.FullEvent()

	// destination
	destinationHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// reply with 500 code
		w.WriteHeader(http.StatusInternalServerError)
	})

	destinationCA := eventingtlstesting.StartServer(ctxDestination, t, 8334, destinationHandler, kncloudevents.WithDrainQuietPeriod(time.Millisecond))
	destination := duckv1.Addressable{
		URL:     apis.HTTPS("localhost:8334"),
		CACerts: &destinationCA,
	}

	// dls
	dlsEventChan := make(chan cloudevents.Event, 10)
	dlsHandler := eventingtlstesting.EventChannelHandler(dlsEventChan)
	dlsReceivedEvents := make([]cloudevents.Event, 0, 10)
	dlsCA := eventingtlstesting.StartServer(ctxDls, t, 8335, dlsHandler, kncloudevents.WithDrainQuietPeriod(time.Millisecond))
	dls := duckv1.Addressable{
		URL:     apis.HTTPS("localhost:8335"),
		CACerts: &dlsCA,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range dlsEventChan {
			dlsReceivedEvents = append(dlsReceivedEvents, event)
		}
	}()

	// send event
	message := binding.ToMessage(&eventToSend)
	info, err := dispatcher.SendMessage(ctx, message, destination, kncloudevents.WithDeadLetterSink(&dls))
	require.Nil(t, err)
	require.Equal(t, 200, info.ResponseCode)

	// check received events
	close(dlsEventChan)
	wg.Wait()

	require.Len(t, dlsReceivedEvents, 1)
	require.Equal(t, eventToSend.ID(), dlsReceivedEvents[0].ID())
	require.Equal(t, eventToSend.Data(), dlsReceivedEvents[0].Data())
}

func getOnlyDomainURL(t *testing.T, shouldSend bool, serverURL string) *apis.URL {
	if shouldSend {
		server, err := url.Parse(serverURL)
		if err != nil {
			t.Errorf("Bad serverURL: %q", serverURL)
		}
		return &apis.URL{
			Host: server.Host,
		}
	}
	return nil
}

type requestValidation struct {
	Host    string
	Headers http.Header
	Body    string
}

type fakeHandler struct {
	t        *testing.T
	response *http.Response
	requests []requestValidation
}

func (f *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Make a copy of the request.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		f.t.Error("Failed to read the request body")
	}
	f.requests = append(f.requests, requestValidation{
		Host:    r.Host,
		Headers: r.Header,
		Body:    string(body),
	})

	// Write the response.
	if f.response != nil {
		for h, vs := range f.response.Header {
			for _, v := range vs {
				w.Header().Add(h, v)
			}
		}
		w.WriteHeader(f.response.StatusCode)
		if _, err := io.Copy(w, f.response.Body); err != nil {
			f.t.Error("Error copying Body:", err)
		}
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}
}

func (f *fakeHandler) popRequest(t *testing.T) requestValidation {
	if len(f.requests) == 0 {
		t.Error("Unable to pop request")
		return requestValidation{
			Host: "MADE UP, no such request",
			Body: "MADE UP, no such request",
		}
	}
	rv := f.requests[0]
	f.requests = f.requests[1:]
	return rv
}

func assertEquality(t *testing.T, replacementURL string, expected, actual requestValidation) {
	t.Helper()
	server, err := url.Parse(replacementURL)
	if err != nil {
		t.Errorf("Bad replacement URL: %q", replacementURL)
	}
	expected.Host = server.Host
	canonicalizeHeaders(expected, actual)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error("Unexpected difference (-want, +got):", diff)
	}
}

func canonicalizeHeaders(rvs ...requestValidation) {
	// HTTP header names are case-insensitive, so normalize them to lower case for comparison.
	for _, rv := range rvs {
		headers := rv.Headers
		for n, v := range headers {
			delete(headers, n)
			n = strings.ToLower(n)
			if unimportantHeaders.Has(n) {
				continue
			}
			if ignoreValueHeaders.Has(n) {
				headers[n] = []string{"ignored-value-header"}
			} else {
				headers[n] = v
			}
		}
	}
}
