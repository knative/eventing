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
package channelwatcher

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/provisioners/fanout"
	"knative.dev/eventing/pkg/provisioners/multichannelfanout"
	"knative.dev/eventing/pkg/provisioners/swappable"
	controllertesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func TestUpdateConfigWatchHandler(t *testing.T) {
	tests := []struct {
		name              string
		channels          []runtime.Object
		clientListError   error
		updateConfigError error
		expectedConfig    *multichannelfanout.Config
	}{
		{
			name:            "Client list error",
			clientListError: fmt.Errorf("Client list error"),
		},
		{
			name:              "update config error",
			updateConfigError: fmt.Errorf("error updating config"),
			expectedConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{},
			},
		},
		{
			name: "Successfully update config",
			channels: []runtime.Object{
				makeChannel("chan-1", "ns-1", "e.f.g.h", makeSubscribable(makeSubscriber("sub1"), makeSubscriber("sub2"))),
				makeChannel("chan-2", "ns-2", "i.j.k.l", makeSubscribable(makeSubscriber("sub3"), makeSubscriber("sub4"))),
				makeChannel("chan-3", "donotwatch", "i.j.k.l", makeSubscribable(makeSubscriber("sub3"), makeSubscriber("sub4"))),
			},
			expectedConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
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
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualConfig := ConfigHolder{}
			watchHandler := UpdateConfigWatchHandler(updateConfigWrapper(&actualConfig, test.updateConfigError), shouldWatch)
			mockClient := getClient(test.channels, getClientMocks(test.clientListError))

			actualError := watchHandler(context.TODO(), mockClient, types.NamespacedName{})
			if actualError != nil {
				if test.clientListError != nil {
					if diff := cmp.Diff(test.clientListError.Error(), actualError.Error()); diff != "" {
						t.Fatalf("Unexpected difference (-want +got): %v", diff)
					}
				}
				if test.updateConfigError != nil {
					if diff := cmp.Diff(test.updateConfigError.Error(), actualError.Error()); diff != "" {
						t.Fatalf("Unexpected difference (-want +got): %v", diff)
					}
				}
			} else {
				if test.clientListError != nil {
					t.Fatalf("Want error %v \n Got nil", test.clientListError)
				}
				if test.updateConfigError != nil {
					t.Fatalf("Want error %v \n Got nil", test.updateConfigError)
				}
			}
			if diff := cmp.Diff(test.expectedConfig, actualConfig.config); diff != "" {
				t.Fatalf("Unexpected difference (-want +got): %v", diff)
			}
		})
	}
}

type ConfigHolder struct {
	config *multichannelfanout.Config
}

func shouldWatch(c *v1alpha1.Channel) bool {
	if c.Namespace == "donotwatch" {
		return false
	}
	return true
}
func updateConfigWrapper(ch *ConfigHolder, returnError error) swappable.UpdateConfig {
	return func(c *multichannelfanout.Config) error {
		ch.config = c
		return returnError
	}
}

func getClient(objs []runtime.Object, mocks controllertesting.Mocks) *controllertesting.MockClient {
	innerClient := fake.NewFakeClient(objs...)
	return controllertesting.NewMockClient(innerClient, mocks)
}

func getClientMocks(listError error) controllertesting.Mocks {
	if listError != nil {
		return controllertesting.Mocks{
			MockLists: []controllertesting.MockList{
				func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
					return controllertesting.Handled, listError
				},
			},
		}
	}
	return controllertesting.Mocks{}
}

func makeChannel(name string, namespace string, hostname string, subscribable *eventingduck.Subscribable) *v1alpha1.Channel {
	c := v1alpha1.Channel{
		Spec: v1alpha1.ChannelSpec{
			Subscribable: subscribable,
		},
	}
	c.Name = name
	c.Namespace = namespace
	c.Status.InitializeConditions()
	c.Status.MarkProvisioned()
	c.Status.MarkProvisionerInstalled()
	c.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   hostname,
	})
	return &c
}
func makeSubscribable(subsriberSpec ...eventingduck.SubscriberSpec) *eventingduck.Subscribable {
	return &eventingduck.Subscribable{
		Subscribers: subsriberSpec,
	}
}

func makeSubscriber(name string) eventingduck.SubscriberSpec {
	return eventingduck.SubscriberSpec{
		SubscriberURI: name + "-suburi",
		ReplyURI:      name + "-replyuri",
	}
}
