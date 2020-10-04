package resources

import (
	"reflect"
	"testing"
)

func TestLabels(t *testing.T) {
	tests := []struct {
		name string
		want map[string]string
	}{
		{
			name: "ok",
			want: map[string]string{
				"eventing.knative.dev/injected": "true",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Labels(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Labels() = %v, want %v", got, tt.want)
			}
		})
	}
}
