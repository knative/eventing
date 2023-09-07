package auth

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

func TestGetAudience(t *testing.T) {

	tests := []struct {
		name       string
		gvk        schema.GroupVersionKind
		objectMeta metav1.ObjectMeta
		want       string
	}{
		{
			name: "should return audience in correct format",
			gvk: schema.GroupVersionKind{
				Group:   "group",
				Version: "version",
				Kind:    "kind",
			},
			objectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
			want: "group/kind/namespace/name",
		},
		{
			name: "should return audience in lower case",
			gvk:  v1.SchemeGroupVersion.WithKind("Broker"),
			objectMeta: metav1.ObjectMeta{
				Name:      "my-Broker",
				Namespace: "my-Namespace",
			},
			want: "eventing.knative.dev/broker/my-namespace/my-broker",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAudience(tt.gvk, tt.objectMeta); got != tt.want {
				t.Errorf("GetAudience() = %v, want %v", got, tt.want)
			}
		})
	}
}
