/*
Copyright 2018 The Knative Authors

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

package fanout

import (
	"errors"
	"fmt"
	"github.com/knative/eventing/pkg/sidecar/clientfactory/fake"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIs2xx(t *testing.T) {
	testCases := []struct {
		sc    int
		is2xx bool
	}{
		{
			sc:    http.StatusContinue,
			is2xx: false,
		},
		{
			sc:    http.StatusOK,
			is2xx: true,
		},
		{
			sc:    http.StatusCreated,
			is2xx: true,
		},
		{
			sc:    http.StatusAccepted,
			is2xx: true,
		},
		{
			sc:    http.StatusNoContent,
			is2xx: true,
		},
		{
			sc:    http.StatusAlreadyReported,
			is2xx: true,
		},
		{
			sc:    http.StatusBadRequest,
			is2xx: false,
		},
		{
			sc:    http.StatusUnauthorized,
			is2xx: false,
		},
		{
			sc:    http.StatusForbidden,
			is2xx: false,
		},
		{
			sc:    http.StatusNotFound,
			is2xx: false,
		},
		{
			sc:    http.StatusMethodNotAllowed,
			is2xx: false,
		},
		{
			sc:    http.StatusNotAcceptable,
			is2xx: false,
		},
		{
			sc:    http.StatusConflict,
			is2xx: false,
		},
		{
			sc:    http.StatusGone,
			is2xx: false,
		},
		{
			sc:    http.StatusPreconditionFailed,
			is2xx: false,
		},
		{
			sc:    http.StatusTooManyRequests,
			is2xx: false,
		},
		{
			sc:    http.StatusInternalServerError,
			is2xx: false,
		},
		{
			sc:    http.StatusServiceUnavailable,
			is2xx: false,
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.sc), func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tc.sc,
			}
			if is2xx(resp) != tc.is2xx {
				t.Errorf("Unexpected is2xx for %v. Expected %v. Actual %v", tc.sc, tc.is2xx, is2xx(resp))
			}
		})
	}
}

func TestDomainToUrl(t *testing.T) {
	expected := "http://foobar/"
	if actual := domainToURL("foobar"); expected != actual {
		t.Errorf("Unexpected domainToURL. Expected: '%v'. Actual: '%v'", expected, actual)
	}
}

func TestServeHTTP(t *testing.T) {
	testCases := []struct {
		name           string
		subs           []Subscription
		cf             *fake.ClientFactory
		origBody       io.Reader
		expectedStatus int
	}{
		{
			name:           "cannot read orig body",
			subs:           []Subscription{},
			origBody:       &failingReader{},
			cf:             &fake.ClientFactory{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "no subs",
			subs:           []Subscription{},
			origBody:       body(""),
			cf:             &fake.ClientFactory{},
			expectedStatus: http.StatusOK,
		},
		{
			name: "one sub -- failed",
			subs: []Subscription{
				{
					ToDomain: "todomain",
				},
			},
			origBody: body(""),
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusNotFound,
						Body:       body(""),
					},
				},
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "one sub -- succeeded",
			subs: []Subscription{
				{
					ToDomain: "todomain",
				},
			},
			origBody: body(""),
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Body:       body(""),
					},
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "two subs -- one fails",
			subs: []Subscription{
				{
					ToDomain: "todomain",
				},
				{
					CallDomain: "calldomain",
				},
			},
			origBody: body(""),
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusForbidden,
						Body:       body(""),
					},
					{
						StatusCode: http.StatusOK,
						Body:       body(""),
					},
				},
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "two subs -- both succeed",
			subs: []Subscription{
				{
					ToDomain: "todomain",
				},
				{
					CallDomain: "calldomain",
				},
			},
			origBody: body(""),
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Body:       body(""),
					},
				},
			},
			expectedStatus: http.StatusOK,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := Config{
				Subscriptions: tc.subs,
			}
			f := NewHandler(zap.NewNop(), config, tc.cf)

			r := httptest.NewRequest("POST", "http://unused/", tc.origBody)

			w := httptest.NewRecorder()
			f.ServeHTTP(w, r)

			if w.Result().StatusCode != tc.expectedStatus {
				t.Errorf("Unexpected status code. Expected %v. Actual %v.", tc.expectedStatus, w.Result().StatusCode)
			}
		})
	}
}

func TestMakeFanoutRequest(t *testing.T) {
	testCases := []struct {
		name              string
		cf                *fake.ClientFactory
		sub               Subscription
		origBody          string
		expectedErr       bool
		expectedNumReq    int
		expectedReqBodies []string
	}{
		{
			name: "`call` fails",
			sub: Subscription{
				CallDomain: "calldomain",
			},
			cf: &fake.ClientFactory{
				RespErr: errors.New("error making request"),
			},
			expectedErr:    true,
			expectedNumReq: 1,
		},
		{
			name: "`call` non-2xx",
			sub: Subscription{
				CallDomain: "calldomain",
			},
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusForbidden,
						Body:       body(""),
					},
				},
			},
			expectedErr:    true,
			expectedNumReq: 1,
		},
		{
			name: "`only `call`",
			sub: Subscription{
				CallDomain: "calldomain",
			},
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Body:       body(""),
					},
				},
			},
			origBody:       "{orig:body}",
			expectedNumReq: 1,
			expectedReqBodies: []string{
				"{orig:body}",
			},
		},
		{
			name: "`to` fails",
			sub: Subscription{
				ToDomain: "todomain",
			},
			cf: &fake.ClientFactory{
				RespErr: errors.New("error making request"),
			},
			expectedErr:    true,
			expectedNumReq: 1,
		},
		{
			name: "`to` non-2xx",
			sub: Subscription{
				ToDomain: "todomain",
			},
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusInternalServerError,
						Body:       body(""),
					},
				},
			},
			expectedErr:    true,
			expectedNumReq: 1,
		},
		{
			name: "only `to`",
			sub: Subscription{
				ToDomain: "todomain",
			},
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Body:       body(""),
					},
				},
			},
			origBody:       "{orig:body}",
			expectedNumReq: 1,
			expectedReqBodies: []string{
				"{orig:body}",
			},
		},
		{
			name: "`call` and `to`",
			sub: Subscription{
				CallDomain: "calldomain",
				ToDomain:   "todomain",
			},
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Body:       body("{call:body}"),
					},
				},
			},
			origBody:       "{orig:body}",
			expectedNumReq: 2,
			expectedReqBodies: []string{
				"{orig:body}",
				"{call:body}",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := fanoutHandler{
				logger:        zap.NewNop(),
				clientFactory: tc.cf,
			}
			r := httptest.NewRequest("POST", "http://unused/", nil)
			err := f.makeFanoutRequest(r, []byte(tc.origBody), tc.sub)
			reqs := tc.cf.GetRequests()
			if numReq := len(reqs); numReq != tc.expectedNumReq {
				t.Errorf("Incorrect number of requests. Expected %v. Actual %v", tc.expectedNumReq, reqs)
			}
			if tc.expectedErr {
				if err == nil {
					t.Errorf("Expected an error making the fanout request. Actual: nil")
				}
				return
			}
			for i := range tc.expectedReqBodies {
				expected := tc.expectedReqBodies[i]
				actualBytes, _ := ioutil.ReadAll(reqs[i].Body)
				actual := string(actualBytes)
				if actual != expected {
					t.Errorf("Unexpected req body. Index %v. Expected %v. Actual %v.", i, expected, actual)
				}
			}
		})
	}
}

func TestMakeRequest(t *testing.T) {
	testCases := []struct {
		name               string
		method             string
		cf                 *fake.ClientFactory
		origHeaders        http.Header
		body               string
		expectedErr        bool
		expectedStatusCode int
		expectedBody       string
	}{
		{
			name:        "bad method",
			method:      "not_a_valid::/method",
			expectedErr: true,
		},
		{
			name:   "bad client factory",
			method: "POST",
			cf: &fake.ClientFactory{
				CreateErr: errors.New("test-induced client creation failure"),
			},
			expectedErr: true,
		},
		{
			name:   "unable to make request",
			method: "POST",
			cf: &fake.ClientFactory{
				RespErr: errors.New("test-induced HTTP Do failure"),
			},
			expectedErr: true,
		},
		{
			name:   "bad status code",
			method: "POST",
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusNotFound,
						Body:       body(""),
					},
				},
			},
			expectedErr: true,
		},
		{
			name:   "good response",
			method: "POST",
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Body:       body("{hello:world}"),
					},
				},
			},
			expectedStatusCode: http.StatusOK,
			expectedBody:       "{hello:world}",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fanoutHandler{
				logger:        zap.NewNop(),
				clientFactory: tc.cf,
			}
			or := &http.Request{
				Method: tc.method,
				Header: tc.origHeaders,
			}
			resp, err := f.makeRequest(or, strings.NewReader(tc.body), "domain")
			if tc.expectedErr {
				if err == nil {
					t.Errorf("Expected an error, but did not get one.")
				}
				return
			} else if err != nil {
				t.Errorf("Unexpected error. %v", err)
			}
			if actual, _ := ioutil.ReadAll(resp.Body); resp.StatusCode != tc.expectedStatusCode || string(actual) != tc.expectedBody {
				t.Errorf("Unexpected response. Expected: %v, '%v'. Actual '%v'.", tc.expectedStatusCode, tc.expectedBody, resp)
			}

		})
	}
}

func body(body string) io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(body))
}

type failingReader struct{}

func (_ *failingReader) Read(p []byte) (int, error) {
	return 0, errors.New("test-induced error reading")
}
