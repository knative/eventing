/*
Copyright 2019 The Knative Authors

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

package utils

import (
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// blockingStart wraps a manager.Runnable that eagerly returns and makes it return only if either a
// non-nil error is returned or the stop channel is closed.
type blockingStart struct {
	logger *zap.Logger
	r      manager.Runnable
}

// NewBlockingStart creates a wrapper around the provided runnable that will block until either the
// runnable returns a non-nil error or the stop channel is closed.
func NewBlockingStart(logger *zap.Logger, runnable manager.Runnable) manager.Runnable {
	return &blockingStart{
		logger: logger,
		r:      runnable,
	}
}

// Start implements manager.Runnable.
func (b *blockingStart) Start(stopCh <-chan struct{}) error {
	err := b.r.Start(stopCh)
	b.logger.Debug("blockingStart finished", zap.Error(err))
	if err != nil {
		return err
	}
	<-stopCh
	b.logger.Debug("blockingStart exiting")
	return nil
}
