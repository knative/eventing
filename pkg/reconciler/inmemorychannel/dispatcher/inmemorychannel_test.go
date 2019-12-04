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

package dispatcher

import (
	"context"
	"testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/reconciler"
	. "knative.dev/eventing/pkg/reconciler/testing"
	reconciletesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS                = "test-namespace"
	imcName               = "test-imc"
	channelServiceAddress = "test-imc-kn-channel.test-namespace.svc.cluster.local"
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	subscribers := []duckv1alpha1.SubscriberSpec{{
		UID:           "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
		Generation:    1,
		SubscriberURI: apis.HTTP("call1"),
		ReplyURI:      apis.HTTP("sink2"),
	}, {
		UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
		Generation:    2,
		SubscriberURI: apis.HTTP("call2"),
		ReplyURI:      apis.HTTP("sink2"),
	}}

	subscriberStatuses := []duckv1alpha1.SubscriberStatus{{
		UID:                "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
		ObservedGeneration: 1,
		Ready:              "True",
	}, {
		UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
		ObservedGeneration: 2,
		Ready:              "True",
	}}

	imcKey := testNS + "/" + imcName
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name:    "updated configuration, no channels",
			Key:     imcKey,
			WantErr: false,
		}, {
			Name: "updated configuration, one channel",
			Key:  imcKey,
			Objects: []runtime.Object{
				reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServiceReady(),
					reconciletesting.WithInMemoryChannelEndpointsReady(),
					reconciletesting.WithInMemoryChannelChannelServiceReady(),
					reconciletesting.WithInMemoryChannelAddress(channelServiceAddress)),
			},
			WantErr: false,
		}, {
			Name: "with subscribers",
			Key:  imcKey,
			Objects: []runtime.Object{
				reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServiceReady(),
					reconciletesting.WithInMemoryChannelEndpointsReady(),
					reconciletesting.WithInMemoryChannelChannelServiceReady(),
					reconciletesting.WithInMemoryChannelSubscribers(subscribers),
					reconciletesting.WithInMemoryChannelAddress(channelServiceAddress)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServiceReady(),
					reconciletesting.WithInMemoryChannelEndpointsReady(),
					reconciletesting.WithInMemoryChannelChannelServiceReady(),
					reconciletesting.WithInMemoryChannelSubscribers(subscribers),
					reconciletesting.WithInMemoryChannelStatusSubscribers(subscriberStatuses),
					reconciletesting.WithInMemoryChannelAddress(channelServiceAddress)),
			}},
			WantErr: false,
		}, {},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:                  reconciler.NewBase(ctx, controllerAgentName, cmw),
			inmemorychannelLister: listers.GetInMemoryChannelLister(),
			// TODO fix
			inmemorychannelInformer: nil,
			dispatcher:              &fakeDispatcher{},
		}
	}, false, logger))
}

type fakeDispatcher struct{}

func (d *fakeDispatcher) UpdateConfig(config *multichannelfanout.Config) error {
	// TODO set error
	return nil
}
