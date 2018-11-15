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

package clusterchannelprovisioner

import (
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "gcp-pubsub-clusterchannelprovisioner-controller"
)

// ProvideController returns a Controller that represents the gcp-pubsub channel Provisioner. It
// reconciles only the ClusterChannelProvisioner.
func ProvideController(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	logger = logger.With(zap.String("controller", controllerAgentName))

	// Setup a new controller to Reconcile ClusterChannelProvisioners that are gcp-pubsub channels.
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

	// Watch ClusterChannelProvisioners.
	err = c.Watch(&source.Kind{
		Type: &eventingv1alpha1.ClusterChannelProvisioner{},
	}, &handler.EnqueueRequestForObject{})
	if err != nil {
		logger.Error("Unable to watch ClusterChannelProvisioners.", zap.Error(err), zap.Any("type", &eventingv1alpha1.ClusterChannelProvisioner{}))
		return nil, err
	}

	return c, nil
}
