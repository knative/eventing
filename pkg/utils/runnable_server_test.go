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

package utils

import (
	"net"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"
)

func NewRunnableServer() (*RunnableServer, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	s := &http.Server{
		Addr: l.Addr().String(),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	rs := &RunnableServer{
		Server:    s,
		ServeFunc: func() error { return s.Serve(l) },
		Logger:    zap.NewNop(),
	}

	return rs, nil
}

func TestRunnableServerWithShutdownTimeout(t *testing.T) {
	rs, err := NewRunnableServer()
	if err != nil {
		t.Fatalf("error creating runnableServer: %v", err)
	}

	rs.ShutdownTimeout = time.Second
	shutdownCh := make(chan struct{})
	rs.Server.RegisterOnShutdown(func() {
		close(shutdownCh)
	})

	stopCh := make(chan struct{})
	go rs.Start(stopCh)
	rsp, err := http.Get("http://" + rs.Addr)
	if err != nil {
		t.Errorf("error making request: %v", err)
	}
	if rsp.StatusCode != 200 {
		t.Errorf("expected response code 200, got %d", rsp.StatusCode)
	}

	close(stopCh)

	select {
	case <-shutdownCh:
		t.Logf("Shutdown correctly")
	case <-time.After(time.Second):
		t.Errorf("Expected server to have shut down by now")
	}
}

func TestRunnableServerWithoutShutdownTimeout(t *testing.T) {
	rs, err := NewRunnableServer()
	if err != nil {
		t.Fatalf("error creating runnableServer: %v", err)
	}

	stopCh := make(chan struct{})
	go rs.Start(stopCh)
	rsp, err := http.Get("http://" + rs.Addr)
	if err != nil {
		t.Errorf("error making request: %v", err)
	}
	if rsp.StatusCode != 200 {
		t.Errorf("expected response code 200, got %d", rsp.StatusCode)
	}

	close(stopCh)
	time.Sleep(time.Millisecond * 10)

	if err := rs.Server.ListenAndServe(); err != http.ErrServerClosed {
		t.Errorf("Expected server to have closed by now: %v", err)
	}
}
