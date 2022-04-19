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

package delivery

import (
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/manifest"

	eventingv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

// WithDeadLetterSink adds the dead letter sink related config to the config.
func WithDeadLetterSink(ref *duckv1.KReference, uri string) manifest.CfgFn {
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

// WithRetry adds the retry related config to the config.
func WithRetry(count int32, backoffPolicy *eventingv1.BackoffPolicyType, backoffDelay *string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["delivery"]; !set {
			cfg["delivery"] = map[string]interface{}{}
		}
		delivery := cfg["delivery"].(map[string]interface{})

		delivery["retry"] = count
		if backoffPolicy != nil {
			delivery["backoffPolicy"] = *backoffPolicy
		}
		if backoffDelay != nil {
			delivery["backoffDelay"] = *backoffDelay
		}
	}
}

// WithTimeout adds the timeout related config to the config.
func WithTimeout(timeout string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["delivery"]; !set {
			cfg["delivery"] = map[string]interface{}{}
		}
		delivery := cfg["delivery"].(map[string]interface{})

		delivery["timeout"] = timeout
	}
}
