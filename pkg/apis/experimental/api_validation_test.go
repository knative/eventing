package experimental

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
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
			flags: map[string]bool{
				flagName: true,
			},
			object:             []string{},
			experimentalFields: []string{"Filter"},
		},
		{
			name:        "enabled flag",
			featureName: flagName,
			flags: map[string]bool{
				flagName: true,
			},
			object: v1.TriggerSpec{
				Broker: "blabla",
				Subscriber: duckv1.Destination{
					URI: apis.HTTP("example.com"),
				},
				Filter: &v1.TriggerFilter{},
			},
			experimentalFields: []string{"Filter"},
		},
		{
			name:        "disabled pointer flag",
			featureName: flagName,
			flags: map[string]bool{
				flagName: false,
			},
			object: v1.TriggerSpec{
				Broker: "blabla",
				Subscriber: duckv1.Destination{
					URI: apis.HTTP("example.com"),
				},
				Filter: &v1.TriggerFilter{},
			},
			experimentalFields: []string{"Filter"},
			wantErrs: &apis.FieldError{
				Message: fmt.Sprintf("Disallowed field because the experimental feature '%s' is disabled", flagName),
				Paths:   []string{"TriggerSpec.Filter"},
			},
		},
		{
			name:        "disabled map flag",
			featureName: flagName,
			flags: map[string]bool{
				flagName: false,
			},
			object: &v1.TriggerFilter{
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
