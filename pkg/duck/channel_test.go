package duck

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
)

func TestNewPhysicalChannel(t *testing.T) {
	imcTypeMeta := metav1.TypeMeta{
		APIVersion: "messaging.knative.dev/v1",
		Kind:       "InMemoryChannel",
	}
	labels := map[string]string{"hello": "world"}
	channelableSpec := eventingduckv1.ChannelableSpec{
		Delivery: &eventingduckv1.DeliverySpec{Retry: pointer.Int32Ptr(3)},
	}

	tests := []struct {
		name    string
		objMeta metav1.ObjectMeta
		opts    []PhysicalChannelOption
		want    *messagingv1.InMemoryChannel
		wantErr bool
	}{
		{
			name: "no options, simple object meta",
			objMeta: metav1.ObjectMeta{
				Name:      "hello",
				Namespace: "world",
			},
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
			},
			want: &messagingv1.InMemoryChannel{
				TypeMeta: imcTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "world",
				},
				Spec: messagingv1.InMemoryChannelSpec{
					ChannelableSpec: channelableSpec,
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
				// Marshal to json and then unmarshal to IMC
				b, err := got.MarshalJSON()
				require.NoError(t, err)

				imc := messagingv1.InMemoryChannel{}
				require.NoError(t, json.Unmarshal(b, &imc))

				require.True(t, equality.Semantic.DeepEqual(&imc, tt.want))
			}
		})
	}
}
