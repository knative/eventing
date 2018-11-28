/*
Copyright 2018 The Knative Authors

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
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	pubsubutil "github.com/knative/eventing/pkg/provisioners/gcppubsub/util"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify itself when creating
	// events.
	controllerAgentName = "gcp-pubsub-channel-controller"
)

// ProvideController returns a Controller that represents the gcp-pubsub channel Provisioner. It
// reconciles only Channels.
func ProvideController(defaultGcpProject string, defaultSecret *corev1.ObjectReference, defaultSecretKey string) func(manager.Manager, *zap.Logger) (controller.Controller, error) {
	return func(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
		// Setup a new controller to Reconcile Channels that belong to this Cluster Channel
		// Provisioner (gcp-pubsub).
		r := &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
			logger:   logger,

			defaultGcpProject:   defaultGcpProject,
			defaultSecret:       defaultSecret,
			defaultSecretKey:    defaultSecretKey,
			pubSubClientCreator: pubsubutil.GcpPubSubClientCreator,
		}
		c, err := controller.New(controllerAgentName, mgr, controller.Options{
			Reconciler: r,
		})
		if err != nil {
			return nil, err
		}

		// Watch Channels.
		err = c.Watch(&source.Kind{
			Type: &eventingv1alpha1.Channel{},
		}, &handler.EnqueueRequestForObject{})
		if err != nil {
			logger.Error("Unable to watch Channels.", zap.Error(err), zap.Any("type", &eventingv1alpha1.Channel{}))
			return nil, err
		}

		// Watch the K8s Services that are owned by Channels.
		err = c.Watch(&source.Kind{
			Type: &corev1.Service{},
		}, &handler.EnqueueRequestForOwner{OwnerType: &eventingv1alpha1.Channel{}, IsController: true})
		if err != nil {
			logger.Error("Unable to watch K8s Services.", zap.Error(err))
			return nil, err
		}

		// Watch the VirtualServices that are owned by Channels.
		err = c.Watch(&source.Kind{
			Type: &istiov1alpha3.VirtualService{},
		}, &handler.EnqueueRequestForOwner{OwnerType: &eventingv1alpha1.Channel{}, IsController: true})
		if err != nil {
			logger.Error("Unable to watch VirtualServices.", zap.Error(err))
			return nil, err
		}

		return c, nil
	}
}
