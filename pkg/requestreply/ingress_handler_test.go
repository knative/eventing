/*
Copyright 2025 The Knative Authors

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

package requestreply

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	requestreplyinformerfake "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/requestreply/fake"
	"knative.dev/eventing/pkg/eventingtls"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	configmapinformerfake "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	reconcilertesting "knative.dev/pkg/reconciler/testing"
)

func TestHandlerServeHttp(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()

	tt := map[string]struct {
		method          string
		uri             string
		body            io.Reader
		headers         http.Header
		expectedHeaders http.Header
		statusCode      int
		requestReplys   []*v1alpha1.RequestReply
		makeReplyEvent  func(e *cloudevents.Event) *cloudevents.Event
		keys            map[types.NamespacedName]map[string][]byte
	}{
		"invalid method PATCH": {
			method:     http.MethodPatch,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: http.StatusMethodNotAllowed,
		},
		"invalid method PUT": {
			method:     http.MethodPut,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: http.StatusMethodNotAllowed,
		},
		"invalid method DELETE": {
			method:     http.MethodDelete,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: http.StatusMethodNotAllowed,
		},
		"invalid method GET": {
			method:     http.MethodGet,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: http.StatusMethodNotAllowed,
		},
		"valid method OPTIONS": {
			method:     http.MethodOptions,
			uri:        "/ns/name",
			body:       strings.NewReader(""),
			statusCode: http.StatusOK,
		},
		"valid event and reply": {
			method:     http.MethodPost,
			uri:        "/default/my-request-reply",
			body:       getValidEvent(),
			statusCode: http.StatusOK,
			requestReplys: []*v1alpha1.RequestReply{
				makeRequestReply("my-request-reply", "default"),
			},
			keys: makeKeysMap(
				addKeysForResource(
					makeRequestReply("my-request-reply", "default"),
					addKey("key", exampleKey),
				),
			),
			makeReplyEvent: copyCorrelationIdToReplyID("correlationid", "replyid"),
		},
		"valid event and reply, multiple RequestReply resources with different keys": {
			method:     http.MethodPost,
			uri:        "/default/my-request-reply",
			body:       getValidEvent(),
			statusCode: http.StatusOK,
			requestReplys: []*v1alpha1.RequestReply{
				makeRequestReply("my-request-reply", "default"),
				makeRequestReply("other-request-reply", "default"),
			},
			keys: makeKeysMap(
				addKeysForResource(
					makeRequestReply("my-request-reply", "default"),
					addKey("key", exampleKey),
				),
				addKeysForResource(
					makeRequestReply("other-request-reply", "default"),
					addKey("key", otherKey),
				),
			),
			makeReplyEvent: copyCorrelationIdToReplyID("correlationid", "replyid"),
		},
		"missing RequestReply": {
			method:     http.MethodPost,
			uri:        "/default/missing-request-reply",
			body:       getValidEvent(),
			statusCode: http.StatusNotFound,
			requestReplys: []*v1alpha1.RequestReply{
				makeRequestReply("my-request-reply", "default"),
			},
			keys: makeKeysMap(
				addKeysForResource(
					makeRequestReply("my-request-reply", "default"),
					addKey("key", exampleKey),
				),
			),
		},
		"missing broker address status annotation": {
			method:     http.MethodPost,
			uri:        "/default/my-request-reply",
			body:       getValidEvent(),
			statusCode: http.StatusInternalServerError,
			requestReplys: []*v1alpha1.RequestReply{
				withUninitializedAnnotations(makeRequestReply("my-request-reply", "default")),
			},
			keys: makeKeysMap(
				addKeysForResource(
					makeRequestReply("my-request-reply", "default"),
					addKey("key", exampleKey),
				),
			),
		},
	}

	for testName, testCase := range tt {
		t.Run(testName, func(t *testing.T) {
			ctx, _ := reconcilertesting.SetupFakeContext(t, setupInformerSelector)

			testHandler := &testServerHandler{
				makeReplyEvent: testCase.makeReplyEvent,
				t:              t,
				uri:            testCase.uri,
			}

			s := httptest.NewServer(testHandler)
			defer s.Close()

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.uri, testCase.body)

			if testCase.headers != nil {
				request.Header = testCase.headers
			} else {
				request.Header.Add(cehttp.ContentType, cloudevents.ApplicationCloudEventsJSON)
			}

			for _, rr := range testCase.requestReplys {
				if rr.Status.Annotations != nil {
					if _, set := rr.Status.Annotations[v1alpha1.RequestReplyBrokerAddressStatusAnnotationKey]; !set {
						rr.Status.Annotations = map[string]string{
							v1alpha1.RequestReplyBrokerAddressStatusAnnotationKey: s.URL,
						}
					}
				}

				requestreplyinformerfake.Get(ctx).Informer().GetStore().Add(rr)
			}

			keyStore := AESKeyStore{}

			for rrName, keyMap := range testCase.keys {
				for keyName, keyValue := range keyMap {
					keyStore.AddAesKey(rrName, keyName, keyValue)
				}
			}

			handler := NewHandler(logger, requestreplyinformerfake.Get(ctx), configmapinformerfake.Get(ctx).Lister().ConfigMaps("ns"), keyStore)

			testHandler.callbackHandler = handler

			handler.ServeHTTP(recorder, request)

			result := recorder.Result()
			assert.Equal(t, testCase.statusCode, result.StatusCode)
		})
	}
}

type testServerHandler struct {
	makeReplyEvent  func(e *cloudevents.Event) *cloudevents.Event
	callbackHandler http.Handler
	uri             string
	t               *testing.T
}

func (ts *testServerHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	// decode incoming event
	event, err := cloudevents.NewEventFromHTTPRequest(request)
	assert.NoError(ts.t, err, "should successfully decode event from forwarded request")

	// make a reply event
	replyEvent := ts.makeReplyEvent(event)

	// send the reply event to the server as a new request (simulating how the RequestReply works)
	replyRequest, _ := cloudevents.NewHTTPRequestFromEvent(context.Background(), ts.uri, *replyEvent)
	// the SDK does not seem to set the request uri, so we set it manually here
	replyRequest.RequestURI = ts.uri
	recorder := httptest.NewRecorder()
	ts.callbackHandler.ServeHTTP(recorder, replyRequest)
}

func setupInformerSelector(ctx context.Context) context.Context {
	ctx = filteredFactory.WithSelectors(ctx, eventingtls.TrustBundleLabelSelector)
	return ctx
}

func makeRequestReply(name, namespace string, opts ...func(rr *v1alpha1.RequestReply)) *v1alpha1.RequestReply {
	rr := &v1alpha1.RequestReply{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "RequestReply",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.RequestReplySpec{
			ReplyAttribute:       "replyid",
			CorrelationAttribute: "correlationid",
		},
		Status: v1alpha1.RequestReplyStatus{
			Status: duckv1.Status{
				Annotations: map[string]string{},
			},
		},
	}

	for _, opt := range opts {
		opt(rr)
	}

	return rr
}

func makeKeysMap(resourceFuncs ...func(map[types.NamespacedName]map[string][]byte)) map[types.NamespacedName]map[string][]byte {
	result := make(map[types.NamespacedName]map[string][]byte)

	for _, f := range resourceFuncs {
		f(result)
	}

	return result
}

func addKeysForResource(rr *v1alpha1.RequestReply, keyFuncs ...func(map[string][]byte) map[string][]byte) func(map[types.NamespacedName]map[string][]byte) {
	return func(m map[types.NamespacedName]map[string][]byte) {
		if m == nil {
			m = make(map[types.NamespacedName]map[string][]byte)
		}

		if m[rr.GetNamespacedName()] == nil {
			m[rr.GetNamespacedName()] = make(map[string][]byte)
		}

		for _, f := range keyFuncs {
			m[rr.GetNamespacedName()] = f(m[rr.GetNamespacedName()])
		}
	}
}

func addKey(keyName string, keyValue []byte) func(map[string][]byte) map[string][]byte {
	return func(m map[string][]byte) map[string][]byte {
		if m == nil {
			m = make(map[string][]byte)
		}
		m[keyName] = keyValue
		return m
	}
}

func copyCorrelationIdToReplyID(correlationIdName, replyIdName string) func(e *cloudevents.Event) *cloudevents.Event {
	return func(e *cloudevents.Event) *cloudevents.Event {
		event := e.Clone()
		event.SetExtension(replyIdName, event.Extensions()[correlationIdName])
		return &event
	}
}

func getValidEvent() io.Reader {
	e := cloudevents.NewEvent()
	e.SetType("type")
	e.SetSource("source")
	e.SetID("1234567890")
	b, _ := e.MarshalJSON()
	return bytes.NewBuffer(b)
}

func withUninitializedAnnotations(rr *v1alpha1.RequestReply) *v1alpha1.RequestReply {
	rr.Status.Annotations = nil
	return rr
}
