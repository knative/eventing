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
	"sync"

	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
)

// TODO: write tests

// ServerManager is intended to be used to manage HTTP and HTTPS servers for a component.
// It relies on the `transport-encryption` feature flag to determine which server(s) should be accepting requests.
// If a server shouldn't be accepting requests, ServerManager will update that server's handler to respond with a 404
//
// disabled: only http server
// permissive: both http and https servers
// strict: only https server
type ServerManager struct {
	httpReceiver  *kncloudevents.HTTPMessageReceiver
	httpsReceiver *kncloudevents.HTTPMessageReceiver
	handler       http.Handler
	cmw           configmap.Watcher
}

func NewServerManager(httpReceiver, httpsReceiver *kncloudevents.HTTPMessageReceiver, handler http.Handler, cmw configmap.Watcher) (*ServerManager, error) {
	if httpReceiver == nil || httpsReceiver == nil {
		return nil, fmt.Errorf("message receiver not provided")
	}

	return &ServerManager{
		httpReceiver:  httpReceiver,
		httpsReceiver: httpsReceiver,
		handler:       handler,
		cmw:           cmw,
	}, nil
}

// Blocking call. Starts the 2 servers
func (s *ServerManager) StartServers(ctx context.Context) (httpError, httpsError error) {
	// start servers
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		httpError = s.httpReceiver.StartListen(ctx, s.handler)
	}()

	go func() {
		defer wg.Done()
		httpsError = s.httpsReceiver.StartListen(ctx, s.handler)
	}()

	<-s.httpReceiver.Ready
	<-s.httpsReceiver.Ready

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		s.updateHandlers(ctx, value.(feature.Flags))
	})
	featureStore.WatchConfigs(s.cmw)

	wg.Wait()
	return
}

func (s *ServerManager) updateHandlers(ctx context.Context, flags feature.Flags) {
	if flags.IsStrictTransportEncryption() {
		logging.FromContext(ctx).Info("transport-encryption: strict. Disabling http handler")
		s.httpReceiver.UpdateHandler(statusNotFoundHandler(ctx))
		s.httpsReceiver.UpdateHandler(s.handler)
	} else if flags.IsPermissiveTransportEncryption() {
		logging.FromContext(ctx).Info("transport-encryption: permissive. Enabling both http and https handlers")
		s.httpReceiver.UpdateHandler(s.handler)
		s.httpsReceiver.UpdateHandler(s.handler)
	} else {
		// disabled mode
		logging.FromContext(ctx).Info("transport encryption: disabled. Disabling https handler")
		s.httpReceiver.UpdateHandler(s.handler)
		s.httpsReceiver.UpdateHandler(statusNotFoundHandler(ctx))
	}
}

func statusNotFoundHandler(ctx context.Context) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
}
