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

package main

import (
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"flag"

	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"

	"knative.dev/eventing/pkg/reconciler/mtbroker"
)

func main() {
	// TODO(https://github.com/knative/eventing/issues/3591): switch back to sharedmain.Main
	clientQPS := flag.Float64("clientQPS", float64(rest.DefaultQPS), "Overrides rest.Config.DefaultQPS.")
	clientBurst := flag.Int("clientBurst", rest.DefaultBurst, "Overrides rest.Config.Burst.")

	// This parses flags.
	cfg := sharedmain.ParseAndGetConfigOrDie()
	cfg.QPS = float32(*clientQPS)
	cfg.Burst = *clientBurst
	sharedmain.MainWithConfig(signals.NewContext(), "mt-broker-controller", cfg, mtbroker.NewController)
}
