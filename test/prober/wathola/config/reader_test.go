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

	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
	"github.com/wavesoftware/go-ensure"

	"io/ioutil"
	"os"
	"path"
	"testing"
)

var id = uuid.New()

func TestReadIfPresent(t *testing.T) {
	// given
	expanded := ensureConfigFileNotPresent()
	data := []byte(`[sender]
address = 'http://default-broker.event-example.svc.cluster.local/'
`)
	ensure.NoError(ioutil.WriteFile(expanded, data, 0644))
	defer func() { ensure.NoError(os.Remove(expanded)) }()

	// when
	ReadIfPresent()

	// then
	assert.Equal(t,
		"http://default-broker.event-example.svc.cluster.local/",
		Instance.Sender.Address)
	assert.Equal(t, DefaultReceiverPort, Instance.Receiver.Port)
	assert.Equal(t, DefaultForwarderPort, Instance.Forwarder.Port)
}

func TestReadIfPresentAndInvalid(t *testing.T) {
	// given
	origLogFatal := logFatal
	defer func() { logFatal = origLogFatal }()
	expanded := ensureConfigFileNotPresent()
	data := []byte(`[sender]
address = 'http://default-broker.event-example.svc.cluster.local/
`)
	ensure.NoError(ioutil.WriteFile(expanded, data, 0644))
	defer func() { ensure.NoError(os.Remove(expanded)) }()
	var errors []string
	logFatal = func(args ...interface{}) {
		errors = append(errors, fmt.Sprint(args))
	}

	// when
	ReadIfPresent()

	// then
	assert.Contains(t, errors, "[(2, 12): unclosed string]")
}

func TestReadIfNotPresent(t *testing.T) {
	// given
	ensureConfigFileNotPresent()

	// when
	ReadIfPresent()

	// then
	assert.Equal(t,
		fmt.Sprintf("http://localhost:%d/", DefaultForwarderPort),
		Instance.Sender.Address)
	assert.Equal(t, DefaultReceiverPort, Instance.Receiver.Port)
	assert.Equal(t, DefaultForwarderPort, Instance.Forwarder.Port)
}

func ensureConfigFileNotPresent() string {
	Instance = defaultValues()
	location = fmt.Sprintf("~/tmp/wathola-%v/config.toml", id.String())
	expanded, err := homedir.Expand(location)
	ensure.NoError(err)
	dir := path.Dir(expanded)
	ensure.NoError(os.MkdirAll(dir, os.ModePerm))
	if _, err := os.Stat(expanded); err == nil {
		ensure.NoError(os.Remove(expanded))
	}

	return expanded
}
