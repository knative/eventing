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
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	corev1 "k8s.io/api/core/v1"
)

const (
	// controllerAgentName is the string used by this controller to identify itself when creating events.
	controllerAgentName = "natss-controller"
)

// ProvideController returns a Controller that represents the NATSS Provisioner.
func ProvideController(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	// Setup a new controller to Reconcile Channels that belong to this Cluster Provisioner
	r := &reconciler{
		recorder: mgr.GetRecorder(controllerAgentName),
		logger:   logger,
	}
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		logger.Error("Unable to create controller.", zap.Error(err))
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
