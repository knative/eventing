package sugar

import (
	"reflect"
	"testing"
)

func TestInjectionDisabledLabels(t *testing.T) {
	tests := []struct {
		name string
		want map[string]string
	}{
		{
			name: "ok",
			want: map[string]string{
				"eventing.knative.dev/injection": "disabled",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InjectionDisabledLabels(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InjectionDisabledLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInjectionEnabledLabels(t *testing.T) {
	tests := []struct {
		name string
		want map[string]string
	}{
		{
			name: "ok",
			want: map[string]string{
				"eventing.knative.dev/injection": "enabled",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InjectionEnabledLabels(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InjectionEnabledLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInjectionLabelKeys(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{
			name: "ok",
			want: []string{
				"eventing.knative.dev/injection",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InjectionLabelKeys(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InjectionLabelKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}
