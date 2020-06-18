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

package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithLivenessCheck(t *testing.T) {

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, livenessURI, nil)

	livenessHandler := WithLivenessCheck(failHandler(t))

	livenessHandler.ServeHTTP(writer, request)

	if writer.Result().StatusCode != http.StatusOK {
		t.Errorf("handler must respond with status code %d to %s requests to path: '%s'", http.StatusOK, http.MethodGet, livenessURI)
	}
}

func TestWithReadinessCheck(t *testing.T) {

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, readinessURI, nil)

	livenessHandler := WithReadinessCheck(failHandler(t))

	livenessHandler.ServeHTTP(writer, request)

	if writer.Result().StatusCode != http.StatusOK {
		t.Errorf("handler must respond with status code %d to %s requests to path: '%s'", http.StatusOK, http.MethodGet, readinessURI)
	}
}

func failHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Error("this handler should not be called")
	})
}
