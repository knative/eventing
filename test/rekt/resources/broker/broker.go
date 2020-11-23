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

package broker

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	eventingv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/k8s"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

type CfgFn func(map[string]interface{})

// WithBrokerClass adds the broker class config to a Broker spec.
func WithBrokerClass(class string) CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["brokerClass"] = class
	}
}

// WithDeadLetterSink adds the dead letter sink related config to a Broker spec.
func WithDeadLetterSink(ref *duckv1.KReference, uri string) CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["delivery"]; !set {
			cfg["delivery"] = map[string]interface{}{}
		}
		delivery := cfg["delivery"].(map[string]interface{})
		if _, set := delivery["deadLetterSink"]; !set {
			delivery["deadLetterSink"] = map[string]interface{}{}
		}
		dls := delivery["deadLetterSink"].(map[string]interface{})
		if uri != "" {
			dls["uri"] = uri
		}
		if ref != nil {
			if _, set := dls["ref"]; !set {
				dls["ref"] = map[string]interface{}{}
			}
			dref := dls["ref"].(map[string]interface{})
			dref["apiVersion"] = ref.APIVersion
			dref["kind"] = ref.Kind
			// Skip namespace.
			dref["name"] = ref.Name
		}
	}
}

// WithRetry adds the retry related config to a Broker spec.
func WithRetry(count int32, backoffPolicy *eventingv1.BackoffPolicyType, backoffDelay *string) CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["delivery"]; !set {
			cfg["delivery"] = map[string]interface{}{}
		}
		delivery := cfg["delivery"].(map[string]interface{})

		delivery["retry"] = count
		if backoffPolicy != nil {
			delivery["backoffPolicy"] = backoffPolicy
		}
		if backoffDelay != nil {
			delivery["backoffDelay"] = backoffDelay
		}
	}
}

// Install will create a Broker resource, augmented with the config fn options.
func Install(name string, opts ...CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}
	for _, fn := range opts {
		fn(cfg)
	}
	return func(ctx context.Context, t *testing.T) {
		if _, err := manifest.InstallLocalYaml(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// IsReady tests to see if a Broker becomes ready within the time given.
func IsReady(name string, interval, timeout time.Duration) feature.StepFn {
	gvr := schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1", Resource: "brokers"}
	return func(ctx context.Context, t *testing.T) {
		env := environment.FromContext(ctx)
		if err := k8s.WaitForResourceReady(ctx, env.Namespace(), name, gvr, interval, timeout); err != nil {
			t.Error("broker did not become ready, ", err)
		}
	}
}

// IsAddressable tests to see if a Broker becomes addressable within the  time
// given.
func IsAddressable(name string, interval, timeout time.Duration) feature.StepFn {
	return func(ctx context.Context, t *testing.T) {
		env := environment.FromContext(ctx)
		c := eventingclient.Get(ctx)
		err := wait.PollImmediate(interval, timeout, func() (bool, error) {
			b, err := c.EventingV1().Brokers(env.Namespace()).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if b.Status.Address.URL == nil {
				return false, fmt.Errorf("broker has no status.address.url, %w", err)
			}
			return true, nil
		})
		if err != nil {
			t.Error("broker has no status.address.url, ", err)
		}
	}
}
