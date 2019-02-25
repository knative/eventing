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
	"github.com/Shopify/sarama"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	common "github.com/knative/eventing/contrib/kafka/pkg/controller"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/system"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "kafka-provisioner-channel-controller"
)

var (
	defaultConfigMapKey = types.NamespacedName{
		Namespace: system.Namespace(),
		Name:      common.DispatcherConfigMapName,
	}
)

type reconciler struct {
	client       client.Client
	recorder     record.EventRecorder
	logger       *zap.Logger
	config       *common.KafkaProvisionerConfig
	configMapKey client.ObjectKey
	// Using a shared kafkaClusterAdmin does not work currently because of an issue with
	// Shopify/sarama, see https://github.com/Shopify/sarama/issues/1162.
	kafkaClusterAdmin sarama.ClusterAdmin
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a Channel controller.
func ProvideController(mgr manager.Manager, config *common.KafkaProvisionerConfig, logger *zap.Logger) (controller.Controller, error) {
	// Setup a new controller to Reconcile Channel.
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: &reconciler{
			recorder:     mgr.GetRecorder(controllerAgentName),
			logger:       logger,
			config:       config,
			configMapKey: defaultConfigMapKey,
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch Channel events and enqueue Channel object key.
	if err := c.Watch(&source.Kind{Type: &eventingv1alpha1.Channel{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// Watch the K8s Services that are owned by Channels.
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{OwnerType: &eventingv1alpha1.Channel{}, IsController: true})
	if err != nil {
		logger.Error("unable to watch K8s Services.", zap.Error(err))
		return nil, err
	}

	// Watch the VirtualServices that are owned by Channels.
	err = c.Watch(&source.Kind{Type: &istiov1alpha3.VirtualService{}}, &handler.EnqueueRequestForOwner{OwnerType: &eventingv1alpha1.Channel{}, IsController: true})
	if err != nil {
		logger.Error("unable to watch VirtualServices.", zap.Error(err))
		return nil, err
	}

	return c, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
