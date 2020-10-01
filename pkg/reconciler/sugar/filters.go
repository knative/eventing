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

package sugar

import (
	"context"

	"github.com/kelseyhightower/envconfig"

	"knative.dev/pkg/logging"
)

// LabelFilterFn defines a function to test if the label set passes the filter.
type LabelFilterFn func(labels map[string]string) bool

type envConfig struct {
	InjectionDefault bool `envconfig:"BROKER_INJECTION_DEFAULT" default:"false"`
}

// LabelFilterFnOrDie returns the filter function to use for filtering labels for injection.
func LabelFilterFnOrDie(ctx context.Context) LabelFilterFn {
	var filter LabelFilterFn

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("annotations namespace controller was unable to process environment: ", err)
	} else if env.InjectionDefault {
		filter = OnByDefault
	} else {
		filter = OffByDefault
	}

	return filter
}

func OnByDefault(labels map[string]string) bool {
	for _, key := range InjectionLabelKeys() {
		if value, found := labels[key]; found {
			switch value {
			case InjectionEnabledLabelValue:
				return true
			case InjectionDisabledLabelValue:
				return false
			}
		}
	}
	// If it was not found and/or not explicitly disabled, then it is defaulted to on.
	return true
}

func OffByDefault(labels map[string]string) bool {
	for _, key := range InjectionLabelKeys() {
		if value, found := labels[key]; found {
			switch value {
			case InjectionEnabledLabelValue:
				return true
			case InjectionDisabledLabelValue:
				return false
			}
		}
	}
	// If it was not found and/or explicitly enabled, then it is defaulted to off.
	return false
}
