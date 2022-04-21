/*
Copyright 2019 The Knative Authors

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

package dispatcher

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing/pkg/channel/fanout"

	. "knative.dev/eventing/pkg/reconciler/testing/v1"
)

func TestReadinessChecker(t *testing.T) {
	// Lister with one in-memory channel.
	ls := NewListers([]runtime.Object{
		NewInMemoryChannel("imc-channel", testNS,
			WithInMemoryChannelDeploymentReady(),
			WithInMemoryChannelServiceReady(),
			WithInMemoryChannelEndpointsReady(),
			WithInMemoryChannelChannelServiceReady(),
			WithInMemoryChannelAddress("fake-address"),
			WithInMemoryChannelDLSUnknown(),
		),
	})

	// Multi-channel handler with 0 handlers.
	hander := newFakeMultiChannelHandler()

	rc := &DispatcherReadyChecker{
		chLister:     ls.GetInMemoryChannelLister(),
		chMsgHandler: hander,
	}

	ts := httptest.NewServer(readinessCheckerHTTPHandler(rc))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	// 1 imc and 0 handlers - dispatcher is not ready to handle events.
	if res.StatusCode != readinessProbeNotReady {
		t.Errorf("Unexpected Readiness probe status. Expected %v. Actual %v.", readinessProbeNotReady, res.StatusCode)
	}

	// Add one handler
	hander.SetChannelHandler("foo", &fanout.FanoutMessageHandler{})

	res, err = http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	// 1 imc and 1 handler - dispatcher is ready.
	if res.StatusCode != readinessProbeReady {
		t.Errorf("Unexpected Readiness probe status. Expected %v. Actual %v.", readinessProbeReady, res.StatusCode)
	}
}
