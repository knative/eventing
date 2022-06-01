package resources

import (
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"testing"
)

func TestNewFilterSubscription(t *testing.T) {
	type args struct {
		branchNumber int
		p            *flowsv1.Parallel
	}
	tests := []struct {
		name string
		args args
		want *messagingv1.Subscription
	}{
		{
			name: "without filter",
			args: args{
				branchNumber: 0,
				p: &flowsv1.Parallel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-parallel",
						Namespace: "test-ns",
					},
					Spec: flowsv1.ParallelSpec{
						ChannelTemplate: &messagingv1.ChannelTemplateSpec{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
							},
							Spec: &runtime.RawExtension{Raw: []byte("{}")},
						},
						Branches: []flowsv1.ParallelBranch{
							{
								Subscriber: duckv1.Destination{URI: apis.HTTP("example.com/subscriber")},
							},
						},
					},
				},
			},
			want: &messagingv1.Subscription{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Subscription",
					APIVersion: "messaging.knative.dev/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-parallel-kn-parallel-filter-0",
					Namespace: "test-ns",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "flows.knative.dev/v1",
							Kind:               "Parallel",
							Name:               "test-parallel",
							Controller:         pointer.Bool(true),
							BlockOwnerDeletion: pointer.Bool(true),
						},
					},
				},
				Spec: messagingv1.SubscriptionSpec{
					Channel: duckv1.KReference{
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "InMemoryChannel",
						Name:       "test-parallel-kn-parallel",
					},
					Subscriber: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "InMemoryChannel",
							Namespace:  "test-ns",
							Name:       "test-parallel-kn-parallel-0",
							APIVersion: "messaging.knative.dev/v1",
						},
					},
				},
			},
		},
		{
			name: "with filter",
			args: args{
				branchNumber: 0,
				p: &flowsv1.Parallel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-parallel",
						Namespace: "test-ns",
					},
					Spec: flowsv1.ParallelSpec{
						ChannelTemplate: &messagingv1.ChannelTemplateSpec{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "messaging.knative.dev/v1",
								Kind:       "InMemoryChannel",
							},
							Spec: &runtime.RawExtension{Raw: []byte("{}")},
						},
						Branches: []flowsv1.ParallelBranch{
							{
								Subscriber: duckv1.Destination{URI: apis.HTTP("example.com/subscriber")},
								Filter:     &duckv1.Destination{URI: apis.HTTP("example.com/filter")},
							},
						},
					},
				},
			},
			want: &messagingv1.Subscription{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Subscription",
					APIVersion: "messaging.knative.dev/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-parallel-kn-parallel-filter-0",
					Namespace: "test-ns",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "flows.knative.dev/v1",
							Kind:               "Parallel",
							Name:               "test-parallel",
							Controller:         pointer.Bool(true),
							BlockOwnerDeletion: pointer.Bool(true),
						},
					},
				},
				Spec: messagingv1.SubscriptionSpec{
					Channel: duckv1.KReference{
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "InMemoryChannel",
						Name:       "test-parallel-kn-parallel",
					},
					Subscriber: &duckv1.Destination{
						URI: apis.HTTP("example.com/filter"),
					},
					Reply: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "InMemoryChannel",
							Namespace:  "test-ns",
							Name:       "test-parallel-kn-parallel-0",
							APIVersion: "messaging.knative.dev/v1",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewFilterSubscription(tt.args.branchNumber, tt.args.p)

			if !cmp.Equal(tt.want, got) {
				t.Errorf("NewFilterSubscription() (-want, +got):\n%s",
					cmp.Diff(tt.want, got))
			}
		})
	}
}
