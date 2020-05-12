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
	//	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"context"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"

	versioned "knative.dev/eventing/pkg/client/clientset/versioned"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	broker "knative.dev/eventing/pkg/upgrader/broker/v0.15.0"
)

func main() {
	ctx := signals.NewContext()
	cfg := sharedmain.ParseAndGetConfigOrDie()
	ctx = context.WithValue(ctx, kubeclient.Key{}, kubernetes.NewForConfigOrDie(cfg))
	ctx = context.WithValue(ctx, eventingclient.Key{}, versioned.NewForConfigOrDie(cfg))
	if err := broker.Upgrade(ctx); err != nil {
		fmt.Printf("Broker Upgrade failed with: %v\n", err)
		os.Exit(1)
	}
}
