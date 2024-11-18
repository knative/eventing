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
	"embed"
	"encoding/json"
	"log"
	"time"

	"github.com/kelseyhightower/envconfig"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/delivery"
)

//go:embed *.yaml
var yaml embed.FS

func GVR() schema.GroupVersionResource {
	gvr, _ := meta.UnsafeGuessKindToResource(GVK())
	return gvr
}

func GVK() schema.GroupVersionKind {
	return schema.ParseGroupKind(EnvCfg.ChannelGK).WithVersion(EnvCfg.ChannelV)
}

func TypeMeta() metav1.TypeMeta {
	gvk := GVK()
	return metav1.TypeMeta{
		Kind:       gvk.Kind,
		APIVersion: gvk.GroupVersion().String(),
	}
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
		if _, err := manifest.InstallYamlFS(ctx, yaml, cfg); err != nil {
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

// HasDeadLetterSinkURI asserts that the Channel has the resolved dead letter sink URI
// in the status.
func HasDeadLetterSinkURI(name string, gvr schema.GroupVersionResource) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		ns := environment.FromContext(ctx).Namespace()
		interval, timeout := environment.PollTimingsFromContext(ctx)
		var lastState *eventingduck.Channelable
		err := wait.Poll(interval, timeout, func() (done bool, err error) {
			ch, err := dynamicclient.Get(ctx).
				Resource(gvr).
				Namespace(ns).
				Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				t.Fatalf("failed to get %s/%s channel: %v", ns, name, err)
			}

			channelable := &eventingduck.Channelable{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(ch.UnstructuredContent(), channelable); err != nil {
				t.Fatal(err)
			}
			lastState = channelable

			if channelable.Status.DeadLetterSinkURI.String() == "" {
				return false, nil
			}

			return true, nil
		})
		if err != nil {
			bytes, _ := json.MarshalIndent(lastState, "", "  ")
			t.Errorf("failed to verify channel has dead letter sink: %w, last state:\n%s", err, string(bytes))
		}
	}
}

// Address returns a Channel's address.
func Address(ctx context.Context, name string, timings ...time.Duration) (*duckv1.Addressable, error) {
	return addressable.Address(ctx, GVR(), name, timings...)
}

// AsRef returns a KRef for a Channel without namespace.
func AsRef(name string) *duckv1.KReference {
	apiVersion, kind := GVK().ToAPIVersionAndKind()
	return &duckv1.KReference{
		Kind:       kind,
		APIVersion: apiVersion,
		Name:       name,
	}
}

// AsRef returns a KRef for a Channel without namespace.
func AsDestinationRef(name string) *duckv1.Destination {
	apiVersion, kind := GVK().ToAPIVersionAndKind()
	return &duckv1.Destination{
		Ref: &duckv1.KReference{
			Kind:       kind,
			APIVersion: apiVersion,
			Name:       name,
		},
	}
}

// WithDeadLetterSink adds the dead letter sink related config to a Subscription spec.
var WithDeadLetterSink = delivery.WithDeadLetterSink

var WithAnnotations = manifest.WithAnnotations

// ValidateAddress validates the address retured by Address
func ValidateAddress(name string, validate addressable.ValidateAddressFn, timings ...time.Duration) feature.StepFn {
	return addressable.ValidateAddress(GVR(), name, validate, timings...)
}
