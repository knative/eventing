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

package sequence

import (
	"context"
	"embed"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/channel_template"
)

//go:embed *.yaml
var yaml embed.FS

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "flows.knative.dev", Version: "v1", Resource: "sequences"}
}

func GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "flows.knative.dev", Version: "v1", Kind: "Sequence"}
}

// Install will create a Sequence resource, augmented with the config fn options.
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

// IsReady tests to see if a Sequence becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(GVR(), name, timing...)
}

// IsAddressable tests to see if a Parallel becomes addressable within the  time
// given.
func IsAddressable(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsAddressable(GVR(), name, timing...)
}

// Address returns a Parallel's address.
func Address(ctx context.Context, name string, timings ...time.Duration) (*duckv1.Addressable, error) {
	return addressable.Address(ctx, GVR(), name, timings...)
}

// AsRef returns a KRef for a Parallel without namespace.
func AsRef(name string) *duckv1.KReference {
	apiVersion, kind := GVK().ToAPIVersionAndKind()
	return &duckv1.KReference{
		Kind:       kind,
		APIVersion: apiVersion,
		Name:       name,
	}
}

// WithStep adds the a step config to a Sequence spec at steps[len].
func WithStep(ref *duckv1.KReference, uri string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["steps"]; !set {
			cfg["steps"] = []map[string]interface{}{}
		}

		step := map[string]interface{}{}
		if uri != "" {
			step["uri"] = uri
		}
		if ref != nil {
			if _, set := step["ref"]; !set {
				step["ref"] = map[string]interface{}{}
			}
			sref := step["ref"].(map[string]interface{})
			sref["apiVersion"] = ref.APIVersion
			sref["kind"] = ref.Kind
			sref["namespace"] = ref.Namespace
			sref["name"] = ref.Name
		}

		steps := cfg["steps"].([]map[string]interface{})
		steps = append(steps, step)

		cfg["steps"] = steps
	}
}

// WithReply adds the top level reply config to a Parallel spec.
func WithReply(ref *duckv1.KReference, uri string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["reply"]; !set {
			cfg["reply"] = map[string]interface{}{}
		}
		reply := cfg["reply"].(map[string]interface{})

		if uri != "" {
			reply["uri"] = uri
		}
		if ref != nil {
			if _, set := reply["ref"]; !set {
				reply["ref"] = map[string]interface{}{}
			}
			rref := reply["ref"].(map[string]interface{})
			rref["apiVersion"] = ref.APIVersion
			rref["kind"] = ref.Kind
			rref["namespace"] = ref.Namespace
			rref["name"] = ref.Name
		}
	}
}

// WithChannelTemplate adds the top level channel references.
func WithChannelTemplate(template channel_template.ChannelTemplate) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["channelTemplate"]; !set {
			cfg["channelTemplate"] = map[string]interface{}{}
		}
		channelTemplate := cfg["channelTemplate"].(map[string]interface{})

		channelTemplate["kind"] = template.Kind
		channelTemplate["apiVersion"] = template.APIVersion

		channelTemplate["spec"] = template.Spec
	}
}

// ValidateAddress validates the address retured by Address
func ValidateAddress(name string, validate addressable.ValidateAddressFn, timings ...time.Duration) feature.StepFn {
	return addressable.ValidateAddress(GVR(), name, validate, timings...)
}
