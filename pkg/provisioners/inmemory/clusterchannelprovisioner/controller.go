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
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "in-memory-channel-controller"
)

func removeNamespace(r *reconcile.Request) {
	// This is done to support reconcilers that reconcile cluster-scoped resources based on watch on a namespace-scoped resource
	// Workaround until https://github.com/kubernetes-sigs/controller-runtime/issues/228 is fix and controller-runtime updated
	// TODO: remove after updating controller-runtime
	r.NamespacedName.Namespace = ""
}

// ProvideController returns an InMemoryChannelProvisioner controller.
func ProvideController(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	logger = logger.With(zap.String("controller", controllerAgentName))

	r, err := eventingreconciler.New(
		&reconciler{},
		logger,
		mgr.GetRecorder(controllerAgentName),
		eventingreconciler.EnableFilter(),
		eventingreconciler.ModifyRequest(eventingreconciler.RequestModifierFunc(removeNamespace)),
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

	// Watch ClusterChannelProvisioners.
	err = c.Watch(
		&source.Kind{
			Type: &eventingv1alpha1.ClusterChannelProvisioner{},
		},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			DeleteFunc: func(event.DeleteEvent) bool {
				return false
			},
		})
	if err != nil {
		logger.Error("Unable to watch ClusterChannelProvisioners.", zap.Error(err), zap.Any("type", &eventingv1alpha1.ClusterChannelProvisioner{}))
		return nil, err
	}

	// Watch the K8s Services that are owned by ClusterChannelProvisioners.
	err = c.Watch(&source.Kind{
		Type: &corev1.Service{},
	}, &handler.EnqueueRequestForOwner{OwnerType: &eventingv1alpha1.ClusterChannelProvisioner{}, IsController: true})
	if err != nil {
		logger.Error("Unable to watch K8s Services.", zap.Error(err))
		return nil, err
	}

	return c, nil
}
