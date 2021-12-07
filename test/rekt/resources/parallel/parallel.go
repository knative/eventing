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

package parallel

import (
	"context"
	"embed"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/eventing/test/rekt/resources/addressable"
)

//go:embed *.yaml
var yaml embed.FS

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "flows.knative.dev", Version: "v1", Resource: "parallels"}
}

func GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "flows.knative.dev", Version: "v1", Kind: "Parallel"}
}

// Install will create a Parallel resource, augmented with the config fn options.
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

// IsReady tests to see if a Parallel becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(GVR(), name, timing...)
}

// IsAddressable tests to see if a Parallel becomes addressable within the  time
// given.
func IsAddressable(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsAddressable(GVR(), name, timing...)
}

// Address returns a Parallel's address.
func Address(ctx context.Context, name string, timings ...time.Duration) (*apis.URL, error) {
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

// WithSubscriberAt adds the subscriber related config to a Parallel spec at branches[`index`].
func WithSubscriberAt(index int, ref *duckv1.KReference, uri string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["branches"]; !set {
			cfg["branches"] = []map[string]interface{}{}
		}

		branches := cfg["branches"].([]map[string]interface{})
		// Grow the array.
		for cap(branches) <= index {
			branches = append(branches, map[string]interface{}{})
		}

		branch := branches[index]
		if _, set := branch["subscriber"]; !set {
			branch["subscriber"] = map[string]interface{}{}
		}
		subscriber := branch["subscriber"].(map[string]interface{})

		if uri != "" {
			subscriber["uri"] = uri
		}
		if ref != nil {
			if _, set := subscriber["ref"]; !set {
				subscriber["ref"] = map[string]interface{}{}
			}
			sref := subscriber["ref"].(map[string]interface{})
			sref["apiVersion"] = ref.APIVersion
			sref["kind"] = ref.Kind
			sref["namespace"] = ref.Namespace
			sref["name"] = ref.Name
		}

		cfg["branches"] = branches
	}
}

// WithFilterAt adds the filter related config to a Parallel spec at branches[`index`].
func WithFilterAt(index int, ref *duckv1.KReference, uri string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["branches"]; !set {
			cfg["branches"] = []map[string]interface{}{}
		}

		branches := cfg["branches"].([]map[string]interface{})
		// Grow the array.
		for cap(branches) <= index {
			branches = append(branches, map[string]interface{}{})
		}

		branch := branches[index]
		if _, set := branch["filter"]; !set {
			branch["filter"] = map[string]interface{}{}
		}
		filter := branch["filter"].(map[string]interface{})

		if uri != "" {
			filter["uri"] = uri
		}
		if ref != nil {
			if _, set := filter["ref"]; !set {
				filter["ref"] = map[string]interface{}{}
			}
			fref := filter["ref"].(map[string]interface{})
			fref["apiVersion"] = ref.APIVersion
			fref["kind"] = ref.Kind
			fref["namespace"] = ref.Namespace
			fref["name"] = ref.Name
		}

		cfg["branches"] = branches
	}
}

// WithReplyAt adds the reply related config to a Parallel spec at branches[`index`].
func WithReplyAt(index int, ref *duckv1.KReference, uri string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["branches"]; !set {
			cfg["branches"] = []map[string]interface{}{}
		}

		branches := cfg["branches"].([]map[string]interface{})
		// Grow the array.
		for cap(branches) <= index {
			branches = append(branches, map[string]interface{}{})
		}

		branch := branches[index]
		if _, set := branch["reply"]; !set {
			branch["reply"] = map[string]interface{}{}
		}
		reply := branch["reply"].(map[string]interface{})

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

		cfg["branches"] = branches
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

type ChannelTemplate struct {
	metav1.TypeMeta

	Spec map[string]interface{}
}

// WithChannelTemplate adds the top level channel references.
func WithChannelTemplate(template ChannelTemplate) manifest.CfgFn {
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
