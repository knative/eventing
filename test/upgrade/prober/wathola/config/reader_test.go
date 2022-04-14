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

package config_test

import (
	"fmt"

	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
)

func TestReadIfPresent(t *testing.T) {
	// given
	contents := `---
sender:
  address: "http://default-broker.event-example.svc/"
`
	errors := withErrorsCaptured(t, func() {
		withConfigContents(t, contents, func() {
			// when
			config.ReadIfPresent()

			// then
			assert.Equal(t,
				"http://default-broker.event-example.svc/",
				config.Instance.Sender.Address)
			assert.Equal(t, config.DefaultReceiverPort, config.Instance.Receiver.Port)
			assert.Equal(t, config.DefaultForwarderPort, config.Instance.Forwarder.Port)
		})
	})
	assert.Empty(t, errors)
}

func TestReadIfPresentAndInvalid(t *testing.T) {
	// given
	contents := `sender:
address = 'http://default-broker.event-example.svc.cluster.local/
`
	errors := withErrorsCaptured(t, func() {
		withConfigContents(t, contents, func() {
			// when
			config.ReadIfPresent()
		})
	})

	// then
	assert.Contains(t, errors, "[error converting YAML to JSON: yaml:"+
		" line 3: could not find expected ':']")
}

func TestReadIfNotPresent(t *testing.T) {
	// given
	configFile := ensureConfigFileNotPresent(t)
	defer func() { assert.NoError(t, os.RemoveAll(configFile)) }()

	// when
	errors := withErrorsCaptured(t, func() {
		config.ReadIfPresent()
	})

	// then
	assert.Equal(t,
		fmt.Sprintf("http://localhost:%d/", config.DefaultForwarderPort),
		config.Instance.Sender.Address)
	assert.Equal(t, config.DefaultReceiverPort, config.Instance.Receiver.Port)
	assert.Equal(t, config.DefaultForwarderPort, config.Instance.Forwarder.Port)
	assert.Empty(t, errors)
}

func TestReadingOfCompositeAddress(t *testing.T) {
	// given
	contents := `---
sender:
  interval: 10000000
  address:
    bootstrapServers: "my-cluster-kafka-bootstrap.kafka.svc:9092"
    topicName: "my-topic"
`
	errors := withErrorsCaptured(t, func() {
		withConfigContents(t, contents, func() {
			// when
			config.ReadIfPresent()

			// then
			address := config.Instance.Sender.Address.(map[string]interface{})
			assert.Equal(t,
				"my-cluster-kafka-bootstrap.kafka.svc:9092",
				address["bootstrapServers"])
			assert.Equal(t,
				"my-topic",
				address["topicName"])
			assert.Equal(t, config.DefaultReceiverPort, config.Instance.Receiver.Port)
			assert.Equal(t, config.DefaultForwarderPort, config.Instance.Forwarder.Port)
			assert.Equal(t,
				zapcore.InfoLevel.String(),
				config.Instance.LogLevel,
			)
		})
	})
	assert.Empty(t, errors)
}

func TestChangingLogLevel(t *testing.T) {
	// given
	contents := `---
log-level: 'DEBUG'
`
	errors := withErrorsCaptured(t, func() {
		withConfigContents(t, contents, func() {
			// when
			config.ReadIfPresent()

			// then
			assert.Equal(t,
				zapcore.DebugLevel.String(),
				config.Instance.LogLevel,
			)
		})
	})

	assert.Empty(t, errors)
}

func withConfigContents(t *testing.T, content string, fn func()) {
	t.Helper()
	configFile := ensureConfigFileNotPresent(t)
	data := []byte(content)
	assert.NoError(t, ioutil.WriteFile(configFile, data, 0644))
	defer func() { assert.NoError(t, os.RemoveAll(configFile)) }()
	fn()
}

func withErrorsCaptured(t *testing.T, fn func()) []string {
	t.Helper()
	origLogFatal := config.LogFatal
	defer func() { config.LogFatal = origLogFatal }()
	var errors []string
	config.LogFatal = func(args ...interface{}) {
		errors = append(errors, fmt.Sprint(args))
	}
	fn()
	return errors
}

func ensureConfigFileNotPresent(t *testing.T) string {
	t.Helper()
	config.Instance = config.Defaults()
	location := fmt.Sprintf("~/tmp/wathola-config-%s.yaml", uuid.NewString())
	expanded, err := homedir.Expand(location)
	assert.NoError(t, err)
	dir := path.Dir(expanded)
	assert.NoError(t, os.MkdirAll(dir, os.ModePerm))
	if _, err = os.Stat(expanded); err == nil {
		assert.NoError(t, os.Remove(expanded))
	}

	t.Setenv(config.LocationEnvVariable, expanded)
	return expanded
}
