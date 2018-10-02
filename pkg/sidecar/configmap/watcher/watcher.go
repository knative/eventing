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

package watcher

import (
	"context"
	"github.com/knative/eventing/pkg/controller/eventing/inmemory/channel"
	"github.com/knative/eventing/pkg/sidecar/configmap/parse"
	"github.com/knative/eventing/pkg/sidecar/swappable"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	watcherName = "config-map-k8s-watcher"
)

// ProvideController returns a flow controller.
func NewWatcher(logger *zap.Logger, mgr manager.Manager, configUpdated swappable.UpdateConfig) (controller.Controller, error) {
	// Setup a new controller to Reconcile ClusterProvisioners that are Stub buses.
	r :=  &reconciler{
		logger: logger,
		configUpdated: configUpdated,
	}
	c, err := controller.New(watcherName, mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		logger.Error("Unable to create controller.", zap.Error(err))
		return nil, err
	}

	// Watch ConfigMaps.
	err = c.Watch(&source.Kind{
		Type: &corev1.ConfigMap{},
	}, &handler.EnqueueRequestForObject{})
	if err != nil {
		logger.Error("Unable to watch ConfiMaps.", zap.Error(err))
		return nil, err
	}
	return c, nil
}

var _ reconcile.Reconciler = &reconciler{}
type reconciler struct {
	logger *zap.Logger
	client     client.Client
	configUpdated swappable.UpdateConfig
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	//TODO use this to store the logger and set a deadline
	ctx := context.TODO()


	// DO NOT SUBMIT !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	if req.Name != channel.ConfigMapName {
		return reconcile.Result{}, nil
	}

	cm := &corev1.ConfigMap{}
	err := r.client.Get(ctx, req.NamespacedName, cm)

	// The ConfigMap may have been deleted since it was added to the workqueue. If so
	// there's nothing to be done.
	if errors.IsNotFound(err) {
		r.logger.Error("ConfigMap not found", zap.Any("request", req))
		return reconcile.Result{}, nil
	}

	// If the ConfigMap exists but could not be retrieved, then we should retry.
	if err != nil {
		r.logger.Error("Could not get ConfigMap",
			zap.Any("request", req),
			zap.Error(err))
		return reconcile.Result{}, err
	}

	config, err := parse.ConfigMapData(r.logger, cm.Data)
	if err != nil {
		r.logger.Error("Could not parse ConfigMap", zap.Error(err),
			zap.Any("configMap.Data", cm.Data))
		return reconcile.Result{}, err
	}

	err = r.configUpdated(config)
	if err != nil {
		r.logger.Error("Unable to update config", zap.Error(err))
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
