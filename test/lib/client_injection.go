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

// This file contains an object which encapsulates k8s clients and other info which are useful for e2e tests.
// Each test case will need to create its own client.

package lib

import (
	"context"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/test"
	"testing"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	apiextclient "knative.dev/pkg/client/injection/apiextensions/client"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

// NewClientFromCtx instantiates and returns several clientsets required for making request to the
// cluster specified by the combination of clusterName and configPath.
func NewClientFromCtx(ctx context.Context, namespace string, t *testing.T) (*Client, error) {
	var err error

	client := &Client{
		Kube:          kubeclient.Get(ctx),
		Eventing:      eventingclient.Get(ctx),
		APIExtensions: apiextclient.Get(ctx),
		Dynamic:       dynamicclient.Get(ctx),
		Namespace:     namespace,
		T:             t,
	}
	client.KubeClient = test.KubeClient{Kube: client.Kube}

	client.Namespace = namespace
	client.T = t
	client.Tracker = NewTracker(t, client.Dynamic)

	// Start informer
	client.EventListener = NewEventListener(client.Kube, client.Namespace, client.T.Logf)
	client.Cleanup(client.EventListener.Stop)

	client.tracingEnv, err = getTracingConfig(client.Kube)
	if err != nil {
		return nil, err
	}

	return client, nil
}
