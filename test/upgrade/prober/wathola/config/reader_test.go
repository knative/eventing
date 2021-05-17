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

package config

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
)

func TestReadIfPresent(t *testing.T) {
	// given
	contents := `[sender]
address = 'http://default-broker.event-example.svc.cluster.local/'
`
	errors := withErrorsCaptured(t, func() {
		withConfigContents(t, contents, func() {
			// when
			ReadIfPresent()

			// then
			assert.Equal(t,
				"http://default-broker.event-example.svc.cluster.local/",
				Instance.Sender.Address)
			assert.Equal(t, DefaultReceiverPort, Instance.Receiver.Port)
			assert.Equal(t, DefaultForwarderPort, Instance.Forwarder.Port)
		})
	})
	assert.Empty(t, errors)
}

func TestReadIfPresentAndInvalid(t *testing.T) {
	// given
	contents := `[sender]
address = 'http://default-broker.event-example.svc.cluster.local/
`
	errors := withErrorsCaptured(t, func() {
		withConfigContents(t, contents, func() {
			// when
			ReadIfPresent()
		})
	})

	// then
	assert.Contains(t, errors, "[toml: literal strings cannot have new lines]")
}

func TestReadIfNotPresent(t *testing.T) {
	// given
	configFile := ensureConfigFileNotPresent(t)
	defer func() { assert.NoError(t, os.RemoveAll(configFile)) }()

	// when
	errors := withErrorsCaptured(t, func() {
		ReadIfPresent()
	})

	// then
	assert.Equal(t,
		fmt.Sprintf("http://localhost:%d/", DefaultForwarderPort),
		Instance.Sender.Address)
	assert.Equal(t, DefaultReceiverPort, Instance.Receiver.Port)
	assert.Equal(t, DefaultForwarderPort, Instance.Forwarder.Port)
	assert.Empty(t, errors)
}

func TestReadingOfCompositeAddress(t *testing.T) {
	// given
	contents := `[sender]
interval = 10000000
  [sender.address]
  bootstrapServers = 'my-cluster-kafka-bootstrap.kafka.svc:9092'
  topicName = 'my-topic'
`
	errors := withErrorsCaptured(t, func() {
		withConfigContents(t, contents, func() {
			// when
			ReadIfPresent()

			// then
			address := Instance.Sender.Address.(map[string]interface{})
			assert.Equal(t,
				"my-cluster-kafka-bootstrap.kafka.svc:9092",
				address["bootstrapServers"])
			assert.Equal(t,
				"my-topic",
				address["topicName"])
			assert.Equal(t, DefaultReceiverPort, Instance.Receiver.Port)
			assert.Equal(t, DefaultForwarderPort, Instance.Forwarder.Port)
			assert.Equal(t,
				zapcore.InfoLevel.String(),
				Instance.LogLevel,
			)
		})
	})
	assert.Empty(t, errors)
}

func TestChangingLogLevel(t *testing.T) {
	// given
	contents := `logLevel = 'DEBUG'
`
	errors := withErrorsCaptured(t, func() {
		withConfigContents(t, contents, func() {
			// when
			ReadIfPresent()

			// then
			assert.Equal(t,
				zapcore.DebugLevel.String(),
				Instance.LogLevel,
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
	origLogFatal := logFatal
	defer func() { logFatal = origLogFatal }()
	var errors []string
	logFatal = func(args ...interface{}) {
		errors = append(errors, fmt.Sprint(args))
	}
	fn()
	return errors
}

func ensureConfigFileNotPresent(t *testing.T) string {
	t.Helper()
	Instance = defaultValues()
	location = fmt.Sprintf("~/tmp/wathola-config-%s.toml", uuid.NewString())
	expanded, err := homedir.Expand(location)
	assert.NoError(t, err)
	dir := path.Dir(expanded)
	assert.NoError(t, os.MkdirAll(dir, os.ModePerm))
	if _, err := os.Stat(expanded); err == nil {
		assert.NoError(t, os.Remove(expanded))
	}

	return expanded
}
