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

package filesystem

import (
	"errors"

	"github.com/fsnotify/fsnotify"
	sidecarconfigmap "github.com/knative/eventing/pkg/sidecar/configmap"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"github.com/knative/eventing/pkg/sidecar/swappable"
	"github.com/knative/pkg/configmap"
	"go.uber.org/zap"
)

const (
	// The mount path of the configMap volume.
	ConfigDir = "/etc/config/fanout_sidecar"
)

// Monitors an attached ConfigMap volume for updated configuration and calls `configUpdated` when
// the value changes.
type configMapWatcher struct {
	logger *zap.Logger
	// The directory to read the configMap from.
	dir string
	// Stop the watcher by closing this channel.
	watcherStopCh chan<- bool

	// The function to call when the configuration is updated.
	configUpdated swappable.UpdateConfig
}

// NewConfigMapWatcher creates a new filesystem.configMapWatcher. The caller is responsible for
// calling Start(<-chan), likely via a controller-runtime Manager.
func NewConfigMapWatcher(logger *zap.Logger, dir string, updateConfig swappable.UpdateConfig) (*configMapWatcher, error) {
	conf, err := readConfigMap(logger, dir)
	if err != nil {
		logger.Error("Unable to read configMap", zap.Error(err))
		return nil, err
	}

	logger.Info("Read initial configMap", zap.Any("conf", conf))

	err = updateConfig(conf)
	if err != nil {
		logger.Error("Unable to use the initial configMap: %v", zap.Error(err))
		return nil, err
	}

	cmw := &configMapWatcher{
		logger:        logger,
		dir:           dir,
		configUpdated: updateConfig,
	}
	return cmw, nil
}

// readConfigMap attempts to read the configMap from the attached volume.
func readConfigMap(logger *zap.Logger, dir string) (*multichannelfanout.Config, error) {
	cm, err := configmap.Load(dir)
	if err != nil {
		return nil, err
	}
	return sidecarconfigmap.NewFanoutConfig(logger, cm)
}

// updateConfig reads the configMap data and calls `configUpdated` with the updated value.
func (cmw *configMapWatcher) updateConfig() {
	conf, err := readConfigMap(cmw.logger, cmw.dir)
	if err != nil {
		cmw.logger.Error("Unable to read the configMap", zap.Error(err))
		return
	}
	err = cmw.configUpdated(conf)
	if err != nil {
		cmw.logger.Error("Unable to update config", zap.Error(err))
		return
	}
}

func (cmw *configMapWatcher) Start(stopCh <-chan struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	err = watcher.Add(cmw.dir)
	if err != nil {
		return err
	}

	for {
		select {
		case _, ok := <-watcher.Events:
			if !ok {
				// Channel closed.
				return errors.New("watcher.Events channel closed")
			}
			cmw.updateConfig()
		case err, ok := <-watcher.Errors:
			if !ok {
				// Channel closed.
				return errors.New("watcher.Errors channel closed")
			}
			cmw.logger.Error("watcher.Errors", zap.Error(err))
		case <-stopCh:
			return watcher.Close()
		}
	}
}
