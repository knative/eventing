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

package subscription

import (
	"context"
	"embed"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/test/rekt/resources/delivery"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "messaging.knative.dev", Version: "v1", Resource: "subscriptions"}
}

// WithChannel adds the channel related config to a Subscription spec.
func WithChannel(ref *duckv1.KReference) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["channel"]; !set {
			cfg["channel"] = map[string]interface{}{}
		}
		ch := cfg["channel"].(map[string]interface{})

		if ref != nil {
			ch["apiVersion"] = ref.APIVersion
			ch["kind"] = ref.Kind
			// skip namespace
			ch["name"] = ref.Name
		}
	}
}

// WithSubscriber adds the subscriber related config to a Subscription spec.
func WithSubscriber(ref *duckv1.KReference, uri string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["subscriber"]; !set {
			cfg["subscriber"] = map[string]interface{}{}
		}
		subscriber := cfg["subscriber"].(map[string]interface{})

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
			// skip namespace
			sref["name"] = ref.Name
		}
	}
}

// WithSubscriber adds the subscriber related config to a Subscription spec.
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
			sref := reply["ref"].(map[string]interface{})
			sref["apiVersion"] = ref.APIVersion
			sref["kind"] = ref.Kind
			// skip namespace
			sref["name"] = ref.Name
		}
	}
}

// WithDeadLetterSink adds the dead letter sink related config to a Subscription spec.
var WithDeadLetterSink = delivery.WithDeadLetterSink

// WithRetry adds the retry related config to a Subscription spec.
var WithRetry = delivery.WithRetry

// Install will create a Subscription resource, augmented with the config fn options.
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

// IsReady tests to see if a Subscription becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(gvr(), name, timing...)
}
