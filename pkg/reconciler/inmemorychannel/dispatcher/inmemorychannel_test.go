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

package controller

import (
	"github.com/knative/eventing/pkg/inmemorychannel"
	"github.com/knative/eventing/pkg/provisioners/multichannelfanout"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	reconciletesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
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

func TestNewController(t *testing.T) {
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()

	// Create informer factories with fake clients. The second parameter sets the
	// resync period to zero, disabling it.
	eventingInformerFactory := informers.NewSharedInformerFactory(eventingClient, 0)

	// Eventing
	imcInformer := eventingInformerFactory.Messaging().V1alpha1().InMemoryChannels()

	dispatcher := &inmemorychannel.InMemoryDispatcher{}

	c := NewController(
		reconciler.Options{
			KubeClientSet:     kubeClient,
			EventingClientSet: eventingClient,
			Logger:            logtesting.TestLogger(t),
		},
		dispatcher,
		imcInformer)

	if c == nil {
		t.Fatalf("Failed to create with NewController")
	}
}

func TestAllCases(t *testing.T) {
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
		}, {},
	}
	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                  reconciler.NewBase(opt, controllerAgentName),
			inmemorychannelLister: listers.GetInMemoryChannelLister(),
			// TODO fix
			inmemorychannelInformer: nil,
			dispatcher:              &fakeDispatcher{},
		}
	},
		false,
	))
}

type fakeDispatcher struct{}

func (d *fakeDispatcher) UpdateConfig(config *multichannelfanout.Config) error {
	// TODO set error
	return nil
}
