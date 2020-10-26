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
	"log"

	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/test/lib/recordevents/logger_vent"
	"knative.dev/eventing/test/lib/recordevents/observer"
	"knative.dev/eventing/test/lib/recordevents/recorder_vent"
	"knative.dev/eventing/test/test_images"
)

func main() {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Error while reading the cfg", err)
	}
	//nolint // nil ctx is fine here, look at the code of EnableInjectionOrDie
	ctx, _ := injection.EnableInjectionOrDie(nil, cfg)
	ctx = test_images.ConfigureLogging(ctx, "recordevents")

	if err := test_images.ConfigureTracing(logging.FromContext(ctx), ""); err != nil {
		logging.FromContext(ctx).Fatal("Unable to setup trace publishing", err)
	}

	obs := observer.NewFromEnv(ctx,
		logger_vent.Logger(logging.FromContext(ctx).Infof),
		recorder_vent.NewFromEnv(ctx),
	)

	err = obs.Start(ctx, kncloudevents.CreateHandler)

	if err != nil {
		logging.FromContext(ctx).Fatal("Error during start", err)
	}

	logging.FromContext(ctx).Info("Closing the recordevents process")
}
