/*
Copyright 2021 The Knative Authors

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

package adapter

import (
	"context"
	"os"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	_ "knative.dev/pkg/system/testing"
)

func TestMain_WithController_DisableHA(t *testing.T) {
	os.Setenv("K_SINK", "http://sink")
	os.Setenv("NAMESPACE", "ns")
	os.Setenv("K_OBSERVABILITY_CONFIG", "{}")
	os.Setenv("K_LOGGING_CONFIG", "error config")
	os.Setenv("MODE", "mymode")
	os.Setenv("disable-ha", "true")

	defer func() {
		os.Unsetenv("K_SINK")
		os.Unsetenv("NAMESPACE")
		os.Unsetenv("K_OBSERVABILITY_CONFIG")
		os.Unsetenv("K_LOGGING_CONFIG")
		os.Unsetenv("MODE")
		os.Unsetenv("disable-ha")
	}()

	ctx, cancel := context.WithCancel(context.TODO())

	ctx, _ = fakekubeclient.With(ctx)

	ctx = WithController(ctx, func(ctx context.Context, adapter Adapter) *controller.Impl {
		r := &myAdapter{}
		return controller.NewContext(ctx, r, controller.ControllerOptions{
			WorkQueueName: "foo",
			Logger:        logging.FromContext(ctx),
		})
	})

	env := ConstructEnvOrDie(func() EnvConfigAccessor { return &myEnvConfig{} })
	done := make(chan bool)
	go func() {
		MainWithInformers(ctx,
			"mycomponent",
			env,
			func(ctx context.Context, processed EnvConfigAccessor, client cloudevents.Client) Adapter {
				env := processed.(*myEnvConfig)

				if env.Mode != "mymode" {
					t.Error("Expected mode mymode, got:", env.Mode)
				}

				if env.Sink != "http://sink" {
					t.Error("Expected sinkURI http://sink, got:", env.Sink)
				}

				if leaderelection.HasLeaderElection(ctx) {
					t.Error("Expected no leader election, but got leader election")
				}
				return &myAdapter{blocking: true}
			})
		done <- true
	}()

	cancel()
	<-done
}

func TestMain_WithControllerHA(t *testing.T) {
	os.Setenv("K_SINK", "http://sink")
	os.Setenv("NAMESPACE", "ns")
	os.Setenv("K_OBSERVABILITY_CONFIG", "{}")
	os.Setenv("K_LOGGING_CONFIG", "error config")
	os.Setenv("MODE", "mymode")

	defer func() {
		os.Unsetenv("K_SINK")
		os.Unsetenv("NAMESPACE")
		os.Unsetenv("K_OBSERVABILITY_CONFIG")
		os.Unsetenv("K_LOGGING_CONFIG")
		os.Unsetenv("MODE")
	}()

	ctx, cancel := context.WithCancel(context.TODO())

	ctx, _ = fakekubeclient.With(ctx)

	ctx = WithHAEnabled(ctx)

	ctx = WithController(ctx, func(ctx context.Context, adapter Adapter) *controller.Impl {
		r := &myAdapter{}
		return controller.NewContext(ctx, r, controller.ControllerOptions{
			WorkQueueName: "foo",
			Logger:        logging.FromContext(ctx),
		})
	})

	env := ConstructEnvOrDie(func() EnvConfigAccessor { return &myEnvConfig{} })
	done := make(chan bool)
	go func() {
		MainWithInformers(ctx,
			"mycomponent",
			env,
			func(ctx context.Context, processed EnvConfigAccessor, client cloudevents.Client) Adapter {
				return &myAdapter{blocking: true}
			})
		done <- true
	}()
	cancel()
	<-done
}
