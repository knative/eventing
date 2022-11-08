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

package main

import (
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"

	"knative.dev/eventing/pkg/reconciler/apiserversource"
	"knative.dev/eventing/pkg/reconciler/channel"
	"knative.dev/eventing/pkg/reconciler/containersource"
	"knative.dev/eventing/pkg/reconciler/eventtype"
	"knative.dev/eventing/pkg/reconciler/parallel"
	"knative.dev/eventing/pkg/reconciler/pingsource"
	"knative.dev/eventing/pkg/reconciler/sequence"
	sourcecrd "knative.dev/eventing/pkg/reconciler/source/crd"
	"knative.dev/eventing/pkg/reconciler/subscription"
	sugarnamespace "knative.dev/eventing/pkg/reconciler/sugar/namespace"
	sugartrigger "knative.dev/eventing/pkg/reconciler/sugar/trigger"
)

func main() {

	ctx := signals.NewContext()

	port := os.Getenv("PROBES_PORT")
	if port == "" {
		port = "8080"
	}

	// sets up liveness and readiness probes.
	server := http.Server{
		ReadTimeout: 5 * time.Second,
		Handler:     http.HandlerFunc(handler),
		Addr:        ":" + port,
	}

	go func() {

		go func() {
			<-ctx.Done()
			_ = server.Shutdown(ctx)
		}()

		// start the web server on port and accept requests
		log.Printf("Readiness and health check server listening on port %s", port)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	sharedmain.MainWithContext(ctx, "controller",
		// Messaging
		channel.NewController,
		subscription.NewController,

		// Eventing
		eventtype.NewController,

		// Flows
		parallel.NewController,
		sequence.NewController,

		// Sources
		apiserversource.NewController,
		pingsource.NewController,
		containersource.NewController,
		// Sources CRD
		sourcecrd.NewController,

		// Sugar
		sugarnamespace.NewController,
		sugartrigger.NewController,
	)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
