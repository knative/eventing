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
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "stub-bus-cluster-provisioner-controller"
)

// ProvideController returns a flow controller.
func ProvideController(mgr manager.Manager, logger *zap.Logger, cpRef *corev1.ObjectReference, syncHttpChannelConfig func() error) (*ConfigAndStopCh, error) {
	logger = logger.With(zap.String("clusterProvisioner", cpRef.Name))

	// Setup a new controller to Reconcile Channels that belong to this Cluster Provisioner (Stub
	// buses).
	r :=  &reconciler{
		recorder: mgr.GetRecorder(controllerAgentName),
		logger: logger,
		stubProvisioner: cpRef,

		syncHttpChannelConfig: syncHttpChannelConfig,
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

	// TODO: Should we watch the K8s service and Istio Virtual Service as well? If they change, we
	// probably should change it back.

	return &ConfigAndStopCh{
		controller: c,
		Config: r.getConfig,
		StopCh: make(chan struct{}, 1),
	}, nil
}

type ConfigAndStopCh struct {
	controller controller.Controller
	Config func() multichannelfanout.Config
	StopCh chan<- struct{}
}

func (cas *ConfigAndStopCh) BackgroundStart() {
	stopCh := make(chan struct{}, 1)
	go func() {
		cas.controller.Start(stopCh)
	}()
}
