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

package channel_impl

import (
	"context"
	"log"
	"time"

	"github.com/kelseyhightower/envconfig"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

func GVR() schema.GroupVersionResource {
	gvr, _ := meta.UnsafeGuessKindToResource(GVK())
	return gvr
}

func GVK() schema.GroupVersionKind {
	return schema.ParseGroupKind(EnvCfg.ChannelGK).WithVersion(EnvCfg.ChannelV)
}

var EnvCfg EnvConfig

type EnvConfig struct {
	ChannelGK string `envconfig:"CHANNEL_GROUP_KIND" default:"InMemoryChannel.messaging.knative.dev" required:"true"`
	ChannelV  string `envconfig:"CHANNEL_VERSION" default:"v1" required:"true"`
}

func init() {
	// Process EventingGlobal.
	if err := envconfig.Process("", &EnvCfg); err != nil {
		log.Fatal("Failed to process env var", err)
	}
}

// Install will create a Channel resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	apiVersion, kind := GVK().ToAPIVersionAndKind()
	cfg := map[string]interface{}{
		"name":       name,
		"kind":       kind,
		"apiVersion": apiVersion,
	}
	for _, fn := range opts {
		fn(cfg)
	}
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallLocalYaml(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// IsReady tests to see if a Channel becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(GVR(), name, timing...)
}

// IsAddressable tests to see if a Channel becomes addressable within the  time
// given.
func IsAddressable(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsAddressable(GVR(), name, timing...)
}

// Address returns a Channel's address.
func Address(ctx context.Context, name string, timings ...time.Duration) (*apis.URL, error) {
	interval, timeout := k8s.PollTimings(ctx, timings)
	var addr *apis.URL
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		var err error
		addr, err = k8s.Address(ctx, GVR(), name)
		if err == nil && addr == nil {
			// keep polling
			return false, nil
		}
		if err != nil {
			if apierrors.IsNotFound(err) {
				// keep polling
				return false, nil
			}
			// seems fatal.
			return false, err
		}
		// success!
		return true, nil
	})
	return addr, err
}
