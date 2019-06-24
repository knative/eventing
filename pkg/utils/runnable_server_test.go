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

package utils

import (
	"net"
	"net/http"
	"testing"
	"time"
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
	}

	return rs, nil
}

func TestRunnableServerCallsShutdown(t *testing.T) {
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
	go func() {
		if err := rs.Start(stopCh); err != nil {
			t.Errorf("Error returned from Start: %v", err)
		}
	}()

	rsp, err := http.Get("http://" + rs.Addr)
	if err != nil {
		t.Errorf("error making request: %v", err)
	}
	if rsp.StatusCode != 200 {
		t.Errorf("expected response code 200, got %d", rsp.StatusCode)
	}

	close(stopCh)

	select {
	case <-time.After(time.Second):
		t.Errorf("Expected server to have shut down by now")
	case <-shutdownCh:
	}
}

func TestRunnableServerShutdownContext(t *testing.T) {
	rs, err := NewRunnableServer()
	if err != nil {
		t.Fatalf("error creating runnableServer: %v", err)
	}

	rs.ShutdownTimeout = time.Millisecond * 10
	rs.Server.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(time.Second * 10)
		w.WriteHeader(http.StatusForbidden)
	})

	stopCh := make(chan struct{})
	stoppedCh := make(chan struct{})
	go func() {
		if err := rs.Start(stopCh); err == nil {
			t.Errorf("Expected context deadline exceeded error from Start but got nil")
		}
		close(stoppedCh)
	}()

	go func() {
		http.Get("http://" + rs.Addr)
	}()

	// Give the request time to start
	time.Sleep(time.Millisecond * 50)
	shutdownCh := time.After(time.Millisecond * 50)

	close(stopCh)

	select {
	case <-shutdownCh:
		t.Errorf("Expected shutdown to complete before the timeout")
	case <-stoppedCh:
	}
}

func TestRunnableServerCallsClose(t *testing.T) {
	rs, err := NewRunnableServer()
	if err != nil {
		t.Fatalf("error creating runnableServer: %v", err)
	}

	stopCh := make(chan struct{})
	go func() {
		if err := rs.Start(stopCh); err != nil {
			t.Errorf("Error returned from Start: %v", err)
		}
	}()

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
