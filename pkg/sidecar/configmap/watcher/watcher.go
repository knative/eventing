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
	sidecarconfigmap "github.com/knative/eventing/pkg/sidecar/configmap"
	"github.com/knative/eventing/pkg/sidecar/swappable"
	"github.com/knative/pkg/configmap"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewWatcher creates a new InformedWatcher that watches the specified ConfigMap and on any change
// that results in a valid multichannelfanout.Config calls configUpdated.
func NewWatcher(logger *zap.Logger, kc kubernetes.Interface, cmNamespace, cmName string, configUpdated swappable.UpdateConfig) (manager.Runnable, error) {
	iw := configmap.NewInformedWatcher(kc, cmNamespace)
	iw.Watch(cmName, func(cm *corev1.ConfigMap) {
		config, err := sidecarconfigmap.NewFanoutConfig(logger, cm.Data)
		if err != nil {
			logger.Error("Could not parse ConfigMap", zap.Error(err),
				zap.Any("configMap.Data", cm.Data))
			return
		}

		err = configUpdated(config)
		if err != nil {
			logger.Error("Unable to update config", zap.Error(err))
			return
		}
	})

	return iw, nil
}
