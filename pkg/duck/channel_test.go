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

package duck

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
)

// I need this just because i need some additional stuff to add in the spec for testing the physical channel spec
type MyCoolChannel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Channel.
	Spec MyCoolChannelSpec `json:"spec,omitempty"`

	// Status represents the current state of the Channel. This data may be out of
	// date.
	// +optional
	Status messagingv1.InMemoryChannelStatus `json:"status,omitempty"`
}

type MyCoolChannelSpec struct {
	// Channel conforms to Duck type Channelable.
	eventingduckv1.ChannelableSpec `json:",inline"`

	MyCoolParameter string `json:"myCoolParameter,omitempty"`
}

func TestNewPhysicalChannel(t *testing.T) {
	imcTypeMeta := metav1.TypeMeta{
		APIVersion: "messaging.knative.dev/v1",
		Kind:       "InMemoryChannel",
	}
	labels := map[string]string{"hello": "world"}
	channelableSpec := eventingduckv1.ChannelableSpec{
		Delivery: &eventingduckv1.DeliverySpec{Retry: pointer.Int32Ptr(3)},
	}
	physicalChannelSpec := runtime.RawExtension{
		Raw: []byte("{\"myCoolParameter\":\"i'm cool\"}"),
	}

	tests := []struct {
		name     string
		objMeta  metav1.ObjectMeta
		opts     []PhysicalChannelOption
		assertFn func(t *testing.T, expected interface{}, got *unstructured.Unstructured)
		want     interface{}
		wantErr  bool
	}{
		{
			name: "no options, simple object meta",
			objMeta: metav1.ObjectMeta{
				Name:      "hello",
				Namespace: "world",
			},
			assertFn: assertIMCEquality,
			want: &messagingv1.InMemoryChannel{
				TypeMeta: imcTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "world",
				},
			},
		},
		{
			name: "no options, with complex object meta",
			objMeta: metav1.ObjectMeta{
				Name:      "hello",
				Namespace: "world",
				Labels:    labels,
			},
			assertFn: assertIMCEquality,
			want: &messagingv1.InMemoryChannel{
				TypeMeta: imcTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "world",
					Labels:    labels,
				},
			},
		},
		{
			name: "with options",
			objMeta: metav1.ObjectMeta{
				Name:      "hello",
				Namespace: "world",
			},
			opts: []PhysicalChannelOption{
				WithChannelableSpec(channelableSpec),
				WithPhysicalChannelSpec(&physicalChannelSpec),
			},
			assertFn: assertMyCoolChannelEquality,
			want: &MyCoolChannel{
				TypeMeta: imcTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "world",
				},
				Spec: MyCoolChannelSpec{
					ChannelableSpec: channelableSpec,
					MyCoolParameter: "i'm cool",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPhysicalChannel(imcTypeMeta, tt.objMeta, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPhysicalChannel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.want != nil {
				tt.assertFn(t, tt.want, got)
			}
		})
	}
}

func assertIMCEquality(t *testing.T, expected interface{}, got *unstructured.Unstructured) {
	// Marshal to json and then unmarshal to IMC
	b, err := got.MarshalJSON()
	require.NoError(t, err)

	imc := messagingv1.InMemoryChannel{}
	require.NoError(t, json.Unmarshal(b, &imc))
	require.True(t, equality.Semantic.DeepEqual(&imc, expected))
}

func assertMyCoolChannelEquality(t *testing.T, expected interface{}, got *unstructured.Unstructured) {
	// Marshal to json and then unmarshal to MyCoolChannel
	b, err := got.MarshalJSON()
	require.NoError(t, err)

	imc := MyCoolChannel{}
	require.NoError(t, json.Unmarshal(b, &imc))
	require.True(t, equality.Semantic.DeepEqual(&imc, expected))
}
