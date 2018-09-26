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

package configmaphandler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/knative/eventing/pkg/sidecar/clientfactory"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"github.com/knative/pkg/configmap"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/yaml"
	"net/http"
	"sync/atomic"
)

const (
	// The mount path of the configMap volume.
	ConfigDir = "/etc/config/fanout_sidecar"
	// The config key that contains all the configuration data.
	multiChannelFanoutConfigKey = "multiChannelFanoutConfig"
)

// http.Handler that monitors an attached ConfigMap volume for updated configuration and updates its
// behavior based on the configuration.
type configMapHandler struct {
	logger *zap.Logger
	// The directory to read the configMap from.
	dir string
	// Stop the watcher by closing this channel. Expected to only be used by tests.
	watcherStopCh chan<- bool
	// The current multichannelfanout.Handler to delegate HTTP requests to. Never use this directly,
	// instead use {get,set}MultiChannelFanoutHandler, which enforces the type we expect.
	fanout atomic.Value
}

// NewHandler creates a new configmaphandler.Handler.
func NewHandler(logger *zap.Logger, dir string, clientFactory clientfactory.ClientFactory) (http.Handler, error) {
	conf, err := readConfigMap(logger, dir)
	if err != nil {
		logger.Error("Unable to read configMap", zap.Error(err))
		return nil, err
	}

	logger.Info("Read initial configMap", zap.Any("conf", conf))

	mcfh, err := multichannelfanout.NewHandler(logger, conf, clientFactory)
	if err != nil {
		logger.Error("Unable to create multichannelfanout.Handler: %v", zap.Error(err))
		return nil, err
	}

	cmh := &configMapHandler{
		logger: logger,
		dir:    dir,
	}
	cmh.setMultiChannelFanoutHandler(mcfh)
	watcherStopCh, err := cmh.startWatcher(dir)
	if err != nil {
		logger.Error("Unable to start the configMap file watcher", zap.Error(err))
		return nil, err
	}
	cmh.watcherStopCh = watcherStopCh
	return cmh, nil
}

// getMultiChannelFanoutHandler gets the current multichannelfanout.Handler to delegate all HTTP
// requests to.
func (cmh *configMapHandler) getMultiChannelFanoutHandler() *multichannelfanout.Handler {
	return cmh.fanout.Load().(*multichannelfanout.Handler)
}

// setMultiChannelFanoutHandler sets a new multichannelfanout.Handler to delegate all subsequent
// HTTP requests to.
func (cmh *configMapHandler) setMultiChannelFanoutHandler(new *multichannelfanout.Handler) {
	cmh.fanout.Store(new)
}

// readConfigMap attempts to read the configMap from the attached volume.
func readConfigMap(logger *zap.Logger, dir string) (multichannelfanout.Config, error) {
	cm, err := configmap.Load(dir)
	if err != nil {
		logger.Error("Unable to read configMap", zap.Error(err))
		return multichannelfanout.Config{}, err
	}

	if _, present := cm[multiChannelFanoutConfigKey]; !present {
		logger.Error("Expected key not found", zap.String("key", multiChannelFanoutConfigKey))
		return multichannelfanout.Config{}, fmt.Errorf("expected key not found: %v", multiChannelFanoutConfigKey)
	}
	jb, err := yaml.ToJSON([]byte(cm[multiChannelFanoutConfigKey]))
	if err != nil {
		logger.Error("Unable to convert multiChannelFanoutConfig to JSON", zap.Error(err))
		return multichannelfanout.Config{}, err
	}
	var conf multichannelfanout.Config
	err = unmarshallJsonDisallowUnknownFields(jb, &conf)
	return conf, err
}

// readConfigMapAndUpdateSubs reads the configMap data and updates configuration of cmh.fanout, if
// it has changed.
//
// Note that this is often called multiple times when the configMap is updated, so it should do its
// best to discard redundant calls.
func (cmh *configMapHandler) readConfigMapAndUpdateConfig() {
	conf, err := readConfigMap(cmh.logger, cmh.dir)
	if err != nil {
		cmh.logger.Error("Unable to read the configMap", zap.Error(err))
		return
	}
	current := cmh.getMultiChannelFanoutHandler()
	if diff := current.ConfigDiff(conf); diff != "" {
		cmh.logger.Info("Updating multiChannelFanout config", zap.String("diff (-old, +new)", diff))
		updated, err := current.CopyWithNewConfig(conf)
		if err != nil {
			cmh.logger.Error("Unable to create updated multichannelfanout.Handler", zap.Error(err))
			return
		}
		cmh.setMultiChannelFanoutHandler(updated)
	} else {
		cmh.logger.Info("fanout config unchanged")
	}
}

// startWatcher starts a background go routine that gets events when the filesystem in configDir is
// changed.
func (cmh *configMapHandler) startWatcher(dir string) (chan<- bool, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	stopCh := make(chan bool)
	go func() {
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					// Channel closed.
					cmh.logger.Error("watcher.Events channel closed") // TODO: Should this be fatal?
					return
				}
				cmh.readConfigMapAndUpdateConfig()
			case err, ok := <-watcher.Errors:
				if !ok {
					// Channel closed.
					cmh.logger.Error("watcher.Errors channel closed") // TODO: Should this be fatal?
					return
				}
				cmh.logger.Error("watcher.Errors", zap.Error(err))
			case _, ok := <-stopCh:
				if !ok {
					// stopCh has been closed
					return
				}
			}
		}
	}()

	return stopCh, watcher.Add(dir)
}

// ServeHTTP delegates all HTTP requests to the current multichannelfanout.Handler.
func (cmh *configMapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Hand work off to the current multi channel fanout handler.
	cmh.getMultiChannelFanoutHandler().ServeHTTP(w, r)
}

// unmarshallJsonDisallowUnknownFields unmarshalls JSON, but unlike json.Unmarshall, will fail if
// given an unknown field (rather than json.Unmarshall's ignoring the unknown field).
func unmarshallJsonDisallowUnknownFields(jb []byte, v interface{}) error {
	d := json.NewDecoder(bytes.NewReader(jb))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
