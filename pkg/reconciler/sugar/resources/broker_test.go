package resources

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
)

func TestMakeBroker(t *testing.T) {
	type args struct {
		namespace string
		name      string
	}
	tests := []struct {
		name string
		args args
		want *v1beta1.Broker
	}{
		{
			name: "ok",
			args: args{
				namespace: "default",
				name:      "df-broker",
			},
			want: &v1beta1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "default",
					Name:        "df-broker",
					Labels:      Labels(),
					Annotations: map[string]string{eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MakeBroker(tt.args.namespace, tt.args.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeBroker() = %v, want %v", got, tt.want)
			}
		})
	}
}
