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
	"embed"
	"log"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/pkg/apis"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

var EnvCfg EnvConfig

type EnvConfig struct {
	BrokerClass        string `envconfig:"BROKER_CLASS" default:"MTChannelBasedBroker" required:"true"`
	BrokerTemplatesDir string `envconfig:"BROKER_TEMPLATES"`
}

func init() {
	// Process EventingGlobal.
	if err := envconfig.Process("", &EnvCfg); err != nil {
		log.Fatal("Failed to process env var", err)
	}
}

func WithEnvConfig() []manifest.CfgFn {
	cfg := []manifest.CfgFn{WithBrokerClass(EnvCfg.BrokerClass)}

	if EnvCfg.BrokerTemplatesDir != "" {
		cfg = append(cfg, WithBrokerTemplateFiles(EnvCfg.BrokerTemplatesDir))
	}
	return cfg
}

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1", Resource: "brokers"}
}

// WithBrokerClass adds the broker class config to a Broker spec.
func WithBrokerClass(class string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["brokerClass"] = class
	}
}

func WithBrokerTemplateFiles(dir string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["__brokerTemplateDir"] = dir
	}
}

// WithConfig adds the specified config map to the Broker spec.
func WithConfig(name string) manifest.CfgFn {
	return func(templateData map[string]interface{}) {
		cfg := make(map[string]interface{})
		cfg["kind"] = "ConfigMap"
		cfg["apiVersion"] = "v1"
		cfg["name"] = name
		templateData["config"] = cfg
	}
}

// WithDeadLetterSink adds the dead letter sink related config to a Broker spec.
var WithDeadLetterSink = delivery.WithDeadLetterSink

// WithRetry adds the retry related config to a Broker spec.
var WithRetry = delivery.WithRetry

// WithTimeout adds the timeout related config to the config.
var WithTimeout = delivery.WithTimeout

// Install will create a Broker resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}
	for _, fn := range opts {
		fn(cfg)
	}

	if dir, ok := cfg["__brokerTemplateDir"]; ok {
		return func(ctx context.Context, t feature.T) {
			if _, err := manifest.InstallYamlFS(ctx, os.DirFS(dir.(string)), cfg); err != nil {
				t.Fatal(err)
			}
		}
	}
	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallYamlFS(ctx, yaml, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// IsReady tests to see if a Broker becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(GVR(), name, timing...)
}

// IsAddressable tests to see if a Broker becomes addressable within the  time
// given.
func IsAddressable(name string, timings ...time.Duration) feature.StepFn {
	return k8s.IsAddressable(GVR(), name, timings...)
}

// Address returns a broker's address.
func Address(ctx context.Context, name string, timings ...time.Duration) (*apis.URL, error) {
	return addressable.Address(ctx, GVR(), name, timings...)
}
