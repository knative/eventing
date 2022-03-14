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

package channel

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"

	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
)

//go:embed *.yaml
var yaml embed.FS

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "messaging.knative.dev", Version: "v1", Resource: "channels"}
}

func GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "messaging.knative.dev", Version: "v1", Kind: "Channel"}
}

// Install will create a Channel resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
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

// Address returns a Channel's address.
func Address(ctx context.Context, name string, timings ...time.Duration) (*apis.URL, error) {
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

// WithTemplate adds channelTemplate to the Channel's config after apply the provided
// options.
func WithTemplate(options ...messagingv1.ChannelTemplateSpecOption) manifest.CfgFn {
	return func(m map[string]interface{}) {
		t := withTemplate(options...)
		channelTemplate := map[string]interface{}{
			"apiVersion": t.APIVersion,
			"kind":       t.Kind,
		}
		m["channelTemplate"] = channelTemplate
		if t.Spec != nil {
			s := map[string]string{}
			bytes, err := t.Spec.MarshalJSON()
			if err != nil {
				panic(fmt.Errorf("failed to marshal spec: %w", err))
			}
			if err := json.Unmarshal(bytes, &s); err != nil {
				panic(fmt.Errorf("failed to unmarshal spec '%s': %v", bytes, err))
			}
			channelTemplate["spec"] = s
		}
	}
}

func withTemplate(options ...messagingv1.ChannelTemplateSpecOption) *messagingv1.ChannelTemplateSpec {
	gvk := channel_impl.GVK()
	t := &messagingv1.ChannelTemplateSpec{
		TypeMeta: metav1.TypeMeta{
			Kind:       gvk.Kind,
			APIVersion: gvk.GroupVersion().String(),
		},
	}
	for _, opt := range options {
		if err := opt(t); err != nil {
			panic(err)
		}
	}
	return t
}
