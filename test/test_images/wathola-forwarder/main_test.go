/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"testing"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/forwarder"
)

func TestForwarderMain(t *testing.T) {
	port := freeport.GetPort()
	config.Instance.LogLevel = zapcore.DebugLevel
	config.Instance.Forwarder.Port = port
	go main()
	cancel := <-forwarder.Canceling
	err := lib.WaitForReadiness(port)
	assert.NoError(t, err)
	assert.NotNil(t, instance)
	cancel()
}
