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

	"github.com/knative/eventing/contrib/natss/pkg/dispatcher/dispatcher"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
)

const (
	// controllerAgentName is the string used by this controller to identify itself when creating events.
	controllerAgentName = "natss-dispatcher"
)

// ProvideController returns a Controller that represents the NATSS Provisioner.
func ProvideController(ss *dispatcher.SubscriptionsSupervisor, mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	logger = logger.With(zap.String("controller", controllerAgentName))

	// Setup a new controller to pull messages from GCP PubSub for Channels that belong to this
	// Cluster Provisioner (gcp-pubsub).
	r, err := eventingreconciler.New(
		&reconciler{subscriptionsSupervisor: ss},
		logger,
		mgr.GetRecorder(controllerAgentName),
		eventingreconciler.EnableFinalizer(finalizerName),
		eventingreconciler.EnableFilter(),
	)
	if err != nil {
		return nil, err
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

	return c, nil
}
