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

package resources

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

func TestMakeBroker(t *testing.T) {
	type args struct {
		namespace string
		name      string
	}
	tests := []struct {
		name string
		args args
		want *v1.Broker
	}{
		{
			name: "ok",
			args: args{
				namespace: "default",
				name:      "df-broker",
			},
			want: &v1.Broker{
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
