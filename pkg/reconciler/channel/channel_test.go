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

package channel

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"

	. "github.com/knative/eventing/pkg/reconciler/testing"
	. "github.com/knative/pkg/reconciler/testing"
)

const (
	testNS          = "testnamespace"
	channeName      = "testchannel"
	provisionerName = "testprovisioner"
)

var (
	provisionerGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "ClusterChannelProvisioner",
	}
)

func init() {
	// Add types to scheme
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		},
		//{ // TODO: there is a bug in the controller, it reconcile for empty provisioner.
		//	Name: "incomplete channel",
		//	Objects: []runtime.Object{
		//		NewChannel(channelName, testNS),
		//	},
		//	Key:    testNS + "/" + channeName,
		//	WantErr: true,
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, "TODO", ""),
		//},
		{
			Name: "unclaimed channel, empty provisioner",
			Objects: []runtime.Object{
				NewChannel(channeName, testNS),
			},
			Key: testNS + "/" + channeName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channeName, testNS,
					WithChannelProvisionerNotFound("", ""), // TODO: THIS IS A BUG, there is no validation checking.
				),
			}},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: %s/%s", testNS, channeName),
				Eventf(corev1.EventTypeWarning, "ChannelUpdateStatusFailed", "Failed to update channel status: %s/%s", testNS, channeName),
			},
		}, {
			Name: "unclaimed channel",
			Objects: []runtime.Object{
				NewChannel(channeName, testNS,
					WithChannelProvisioner(provisionerGVK, provisionerName),
				),
			},
			Key: testNS + "/" + channeName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channeName, testNS,
					WithChannelProvisioner(provisionerGVK, provisionerName),
					// Status Update:
					WithChannelProvisionerNotFound(provisionerName, provisionerGVK.Kind),
				),
			}},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: %s/%s", testNS, channeName),
			},
		}, {
			Name: "controller defaulted channel",
			Objects: []runtime.Object{
				NewChannel(channeName, testNS,
					WithInitChannelConditions,
				),
			},
			Key:     testNS + "/" + channeName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: %s/%s", testNS, channeName),
			},
		}, {
			Name: "valid claimed channel",
			Objects: []runtime.Object{
				NewChannel(channeName, testNS,
					WithChannelProvisioner(provisionerGVK, provisionerName),
					WithInitChannelConditions,
				),
			},
			Key:     testNS + "/" + channeName,
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ChannelReconciled", "Channel reconciled: %s/%s", testNS, channeName),
			},
		}, {
			Name: "channel deleted is no-op",
			Objects: []runtime.Object{
				NewChannel(channeName, testNS,
					WithInitChannelConditions,
					WithChannelDeleted,
				),
			},
			Key: testNS + "/" + channeName,
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:          reconciler.NewBase(opt, controllerAgentName),
			channelLister: listers.GetChannelLister(),
		}
	},
		false,
	))
}
