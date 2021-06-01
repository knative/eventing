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

package feature_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"

	. "knative.dev/eventing/pkg/apis/feature"
)

const flagName = "my-flag"

func TestValidateAPIFields(t *testing.T) {
	tests := []struct {
		name               string
		flags              Flags
		featureName        string
		object             interface{}
		experimentalFields []string
		wantErrs           *apis.FieldError
	}{
		{
			name:        "invalid input",
			featureName: flagName,
			flags: map[string]Flag{
				flagName: Enabled,
			},
			object:             []string{},
			experimentalFields: []string{"Filter"},
		},
		{
			name:        "enabled flag",
			featureName: flagName,
			flags: map[string]Flag{
				flagName: Enabled,
			},
			object: eventingv1.TriggerSpec{
				Broker: "blabla",
				Subscriber: duckv1.Destination{
					URI: apis.HTTP("example.com"),
				},
				Filter: &eventingv1.TriggerFilter{},
			},
			experimentalFields: []string{"Filter"},
		},
		{
			name:        "disabled pointer flag",
			featureName: flagName,
			flags: map[string]Flag{
				flagName: Disabled,
			},
			object: eventingv1.TriggerSpec{
				Broker: "blabla",
				Subscriber: duckv1.Destination{
					URI: apis.HTTP("example.com"),
				},
				Filter: &eventingv1.TriggerFilter{},
			},
			experimentalFields: []string{"Filter"},
			wantErrs: &apis.FieldError{
				Message: fmt.Sprintf("Disallowed field because the experimental feature '%s' is disabled", flagName),
				Paths:   []string{"TriggerSpec.Filter"},
			},
		},
		{
			name:        "disabled nested string flag",
			featureName: flagName,
			flags: map[string]Flag{
				flagName: Disabled,
			},
			object: eventingv1.TriggerSpec{
				Broker: "blabla",
				Subscriber: duckv1.Destination{
					Ref: &duckv1.KReference{
						Namespace: "abc",
					},
				},
			},
			experimentalFields: []string{"Subscriber.Ref.Namespace"},
			wantErrs: &apis.FieldError{
				Message: fmt.Sprintf("Disallowed field because the experimental feature '%s' is disabled", flagName),
				Paths:   []string{"Subscriber.Ref.Namespace"},
			},
		},
		{
			name:        "enabled nested string flag",
			featureName: flagName,
			flags: map[string]Flag{
				flagName: Enabled,
			},
			object: eventingv1.TriggerSpec{
				Broker: "blabla",
				Subscriber: duckv1.Destination{
					Ref: &duckv1.KReference{
						Namespace: "abc",
					},
				},
			},
			experimentalFields: []string{"Subscriber.Ref.Namespace"},
		},
		{
			name:        "disabled map flag",
			featureName: flagName,
			flags: map[string]Flag{
				flagName: Disabled,
			},
			object: &eventingv1.TriggerFilter{
				Attributes: map[string]string{},
			},
			experimentalFields: []string{"Attributes"},
			wantErrs: &apis.FieldError{
				Message: fmt.Sprintf("Disallowed field because the experimental feature '%s' is disabled", flagName),
				Paths:   []string{"TriggerFilter.Attributes"},
			},
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ToContext(context.Background(), tt.flags)

			res := ValidateAPIFields(ctx, tt.featureName, tt.object, tt.experimentalFields...)
			if tt.wantErrs == nil {
				require.Nil(t, res)
			} else {
				require.Error(t, res, tt.wantErrs.Error())
			}
		})
	}
}

func TestValidateAnnotations(t *testing.T) {
	tests := []struct {
		name               string
		flags              Flags
		featureName        string
		object             metav1.Object
		experimentalFields []string
		wantErrs           *apis.FieldError
	}{{
		name:        "enabled flag",
		featureName: flagName,
		flags: map[string]Flag{
			flagName: Enabled,
		},
		object: &eventingv1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"dev.knative/myfancyannotation": "blabla",
				},
			},
		},
		experimentalFields: []string{"dev.knative/myfancyannotation"},
	},
		{
			name:        "disabled flag",
			featureName: flagName,
			flags: map[string]Flag{
				flagName: Disabled,
			},
			object: &eventingv1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"dev.knative/myfancyannotation": "blabla",
					},
				},
			},
			experimentalFields: []string{"dev.knative/myfancyannotation"},
			wantErrs: &apis.FieldError{
				Message: fmt.Sprintf("Disallowed field because the experimental feature '%s' is disabled", flagName),
				Paths:   []string{"dev.knative/myfancyannotation"},
			},
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ToContext(context.Background(), tt.flags)

			res := ValidateAnnotations(ctx, tt.featureName, tt.object, tt.experimentalFields...)
			if tt.wantErrs == nil {
				require.Nil(t, res)
			} else {
				require.Error(t, res, tt.wantErrs.Error())
			}
		})
	}
}
