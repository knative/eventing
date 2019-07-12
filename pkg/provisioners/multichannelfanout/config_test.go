/*
Copyright 2019 The Knative Authors

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

package multichannelfanout

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners/fanout"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func TestNewConfigFromChannels(t *testing.T) {
	tests := []struct {
		name     string
		channels []v1alpha1.Channel
		expected *Config
	}{
		{
			name:     "empty channels list",
			channels: []v1alpha1.Channel{},
			expected: &Config{
				ChannelConfigs: []ChannelConfig{},
			},
		}, {
			name: "one channel with no subscribers",
			channels: []v1alpha1.Channel{
				makeChannel("chan-1", "ns-1", "a.b.c.d", nil),
			},
			expected: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Name:      "chan-1",
						Namespace: "ns-1",
						HostName:  "a.b.c.d",
					},
				},
			},
		}, {
			name: "multiple channels with subscribers",
			channels: []v1alpha1.Channel{
				makeChannel("chan-1", "ns-1", "e.f.g.h", makeSubscribable(makeSubscriber("sub1"), makeSubscriber("sub2"))),
				makeChannel("chan-2", "ns-2", "i.j.k.l", makeSubscribable(makeSubscriber("sub3"), makeSubscriber("sub4"))),
			},
			expected: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Name:      "chan-1",
						Namespace: "ns-1",
						HostName:  "e.f.g.h",
						FanoutConfig: fanout.Config{
							AsyncHandler: true,
							Subscriptions: []eventingduck.SubscriberSpec{
								makeSubscriber("sub1"),
								makeSubscriber("sub2"),
							},
						},
					}, {
						Name:      "chan-2",
						Namespace: "ns-2",
						HostName:  "i.j.k.l",
						FanoutConfig: fanout.Config{
							AsyncHandler: true,
							Subscriptions: []eventingduck.SubscriberSpec{
								makeSubscriber("sub3"),
								makeSubscriber("sub4"),
							},
						},
					},
				},
			},
		}, {
			name: "in-memory provisioner -- async",
			channels: []v1alpha1.Channel{
				withProvisioner(
					makeChannel("chan-1", "ns-1", "a.b.c.d", makeSubscribable(makeSubscriber("sub1"))),
					&v1.ObjectReference{
						Name: "in-memory",
					}),
			},
			expected: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Name:      "chan-1",
						Namespace: "ns-1",
						HostName:  "a.b.c.d",
						FanoutConfig: fanout.Config{
							AsyncHandler: true,
							Subscriptions: []eventingduck.SubscriberSpec{
								makeSubscriber("sub1"),
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := NewConfigFromChannels(test.channels)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Fatalf("Unexpected difference (-want +got): %v", diff)
			}
		})
	}
}

func makeChannel(name, namespace, hostname string, subscribable *eventingduck.Subscribable) v1alpha1.Channel {
	c := v1alpha1.Channel{
		Spec: v1alpha1.ChannelSpec{
			Subscribable: subscribable,
		},
		Status: v1alpha1.ChannelStatus{
			Address: duckv1alpha1.Addressable{
				Addressable: duckv1beta1.Addressable{
					URL: &apis.URL{
						Scheme: "http",
						Host:   hostname,
					},
				},
				Hostname: hostname,
			},
		},
	}
	c.Name = name
	c.Namespace = namespace
	return c
}

func withProvisioner(c v1alpha1.Channel, p *v1.ObjectReference) v1alpha1.Channel {
	c.Spec.Provisioner = p
	return c
}

func makeSubscribable(subscriberSpec ...eventingduck.SubscriberSpec) *eventingduck.Subscribable {
	return &eventingduck.Subscribable{
		Subscribers: subscriberSpec,
	}
}

func makeSubscriber(name string) eventingduck.SubscriberSpec {
	return eventingduck.SubscriberSpec{
		SubscriberURI: name + "-suburi",
		ReplyURI:      name + "-replyuri",
	}
}
