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

package controller

import (
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "kafka-provisioner-controller"
	// ConfigMapName is the name of the ConfigMap in the knative-eventing namespace that contains
	// the subscription information for all kafka Channels. The Provisioner writes to it and the
	// Dispatcher reads from it.
	DispatcherConfigMapName = "kafka-channel-dispatcher"
)

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
	logger   *zap.Logger
	config   *KafkaProvisionerConfig
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a Provisioner controller.
func ProvideController(mgr manager.Manager, config *KafkaProvisionerConfig, logger *zap.Logger) (controller.Controller, error) {
	// Setup a new controller to Reconcile Provisioners.
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
			logger:   logger,
			config:   config,
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch ClusterChannelProvisioner events and enqueue ClusterChannelProvisioner object key.
	if err := c.Watch(&source.Kind{Type: &v1alpha1.ClusterChannelProvisioner{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// Watch the K8s Services that are owned by ClusterChannelProvisioners.
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{OwnerType: &eventingv1alpha1.ClusterChannelProvisioner{}, IsController: true})
	if err != nil {
		logger.Error("unable to watch K8s Services.", zap.Error(err))
		return nil, err
	}

	return c, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
