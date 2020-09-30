/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"go.uber.org/zap"

	"knative.dev/eventing/test/lib/dropevents"
	"knative.dev/eventing/test/lib/recordevents/observer"
	"knative.dev/eventing/test/lib/recordevents/recorder_vent"
	"knative.dev/eventing/test/lib/recordevents/writer_vent"
	"knative.dev/eventing/test/test_images"
)

func main() {
	ctx := context.TODO()

	logger, _ := zap.NewDevelopment()
	if err := test_images.ConfigureTracing(logger.Sugar(), ""); err != nil {
		log.Fatalf("Unable to setup trace publishing: %v", err)
	}

	obs := observer.NewFromEnv(
		recorder_vent.NewFromEnv(ctx),
		writer_vent.NewEventLog(ctx, os.Stdout),
	)

	var err error
	algorithm, ok := os.LookupEnv(dropevents.SkipAlgorithmKey)
	if ok {
		skipper := dropevents.SkipperAlgorithm(algorithm)
		counter := dropevents.CounterHandler{
			Skipper: skipper,
		}
		err = obs.Start(ctx, func(handler http.Handler) http.Handler {
			return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if counter.Skip() {
					writer.WriteHeader(http.StatusConflict)
					return
				}
				handler.ServeHTTP(writer, request)
			})
		})
	} else {
		err = obs.Start(ctx)
	}

	if err != nil {
		log.Fatalf("Problem with start %s", err)
	}
}
