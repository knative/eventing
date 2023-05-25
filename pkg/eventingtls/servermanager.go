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

package eventingtls

import (
	"context"
	"fmt"
	"net/http"

	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
)

// ServerManager is intended to be used to manage HTTP and HTTPS servers for a component.
// It relies on the `transport-encryption` feature flag to determine which server(s) should be accepting requests.
// If a server shouldn't be accepting requests, ServerManager will update that server's handler to respond with a 404
//
// disabled: only http server
// permissive: both http and https servers
// strict: only https server
type ServerManager struct {
	httpReceiver  Receiver
	httpsReceiver Receiver
	handler       http.Handler
	cmw           configmap.Watcher
	featureStore  *feature.Store
}

type Receiver interface {
	StartListen(context.Context, http.Handler) error
}

func NewServerManager(ctx context.Context, httpReceiver, httpsReceiver Receiver, handler http.Handler, cmw configmap.Watcher) (*ServerManager, error) {
	if httpReceiver == nil || httpsReceiver == nil {
		return nil, fmt.Errorf("message receiver not provided")
	}

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"))
	featureStore.WatchConfigs(cmw)

	return &ServerManager{
		httpReceiver:  httpReceiver,
		httpsReceiver: httpsReceiver,
		handler:       handler,
		cmw:           cmw,
		featureStore:  featureStore,
	}, nil
}

// Blocking call. Starts the 2 servers
func (s *ServerManager) StartServers(ctx context.Context) error {
	// start servers
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		if err := s.httpReceiver.StartListen(ctx, s.httpHandler()); err != nil {
			errCh <- err
		}
	}()

	go func() {
		if err := s.httpsReceiver.StartListen(ctx, s.httpsHandler()); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

func (s *ServerManager) httpHandler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		flags := s.featureStore.Load()
		if flags.IsStrictTransportEncryption() {
			// As flag updates are eventually consistent across all components,
			// we want a retryable error. A 404 seemed the most reasonable (400
			// is not retryable).
			response.WriteHeader(http.StatusNotFound)
			return
		}
		s.handler.ServeHTTP(response, request)
	})
}

func (s *ServerManager) httpsHandler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		flags := s.featureStore.Load()
		if flags.IsDisabledTransportEncryption() {
			// As flag updates are eventually consistent across all components,
			// we want a retryable error. A 404 seemed the most reasonable (400
			// is not retryable).
			response.WriteHeader(http.StatusNotFound)
			return
		}
		s.handler.ServeHTTP(response, request)
	})
}
