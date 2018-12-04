/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package receiver

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/fakepubsub"
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/testcreds"
)

const (
	gcpProject = "my-gcp-project"

	validMessage = `{
    "cloudEventsVersion" : "0.1",
    "eventType" : "com.example.someevent",
    "eventTypeVersion" : "1.0",
    "source" : "/mycontext",
    "eventID" : "A234-1234-1234",
    "eventTime" : "2018-04-05T17:31:00Z",
    "extensions" : {
      "comExampleExtension" : "value"
    },
    "contentType" : "text/xml",
    "data" : "<much wow=\"xml\"/>"
}`
)

func TestReceiver(t *testing.T) {
	testCases := map[string]struct {
		initialState []runtime.Object
		pubSubData   fakepubsub.CreatorData
		expectedErr  bool
	}{
		"credential fails": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithInvalidCreds(),
			},
			expectedErr: true,
		},
		"PubSub client fails": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithCreds(),
			},
			pubSubData: fakepubsub.CreatorData{
				ClientCreateErr: errors.New("testInducedError"),
			},
			expectedErr: true,
		},
		"Publish fails": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithCreds(),
			},
			pubSubData: fakepubsub.CreatorData{
				ClientData: fakepubsub.ClientData{
					TopicData: fakepubsub.TopicData{
						Publish: fakepubsub.PublishResultData{
							Err: errors.New("testInducedError"),
						},
					},
				},
			},
			expectedErr: true,
		},
		"Publish succeeds": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithCreds(),
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, mr := New(
				zap.NewNop(),
				fake.NewFakeClient(tc.initialState...),
				fakepubsub.Creator(tc.pubSubData),
				gcpProject,
				testcreds.Secret,
				testcreds.SecretKey)
			resp := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", strings.NewReader(validMessage))
			mr.HandleRequest(resp, req)
			if tc.expectedErr {
				if resp.Result().StatusCode >= 200 && resp.Result().StatusCode < 300 {
					t.Errorf("Expected an error. Actual: %v", resp.Result())
				}
			} else {
				if resp.Result().StatusCode < 200 || resp.Result().StatusCode >= 300 {
					t.Errorf("Expected success. Actual: %v", resp.Result())
				}
			}
		})
	}
}
