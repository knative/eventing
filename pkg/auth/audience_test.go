package auth

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/kmeta"
)

func TestGetAudience(t *testing.T) {

	tests := []struct {
		name string
		obj  kmeta.Accessor
		want string
	}{
		{
			name: "should return audience in correct format",
			obj: &v1.Broker{
				TypeMeta: metav1.TypeMeta{
					Kind:       "kind",
					APIVersion: "group/version",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
			},
			want: "group/kind/namespace/name",
		},
		{
			name: "should return audience in lower case",
			obj: &v1.Broker{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Broker",
					APIVersion: "Eventing.knative.dev/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-Broker",
					Namespace: "my-Namespace",
				},
			},
			want: "eventing.knative.dev/broker/my-namespace/my-broker",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAudience(tt.obj); got != tt.want {
				t.Errorf("GetAudience() = %v, want %v", got, tt.want)
			}
		})
	}
}
