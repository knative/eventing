/*
 * Copyright 2021 The Knative Authors
 *
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

package sender_test

import (
	"fmt"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"knative.dev/eventing/test/upgrade/prober/wathola/sender"
)

func TestRegisterEventSender(t *testing.T) {
	sender.RegisterEventSender(testEventSender{})
	c := testConfig{
		topic:   "sample",
		servers: "example.org:9290",
		valid:   true,
	}
	ce := sender.NewCloudEvent(nil, "cetype")
	err := sender.SendEvent(ce, c)

	assert.NoError(t, err)
}

type testEventSender struct{}

type testConfig struct {
	topic   string
	servers string
	valid   bool
}

func (t testEventSender) Supports(endpoint interface{}) bool {
	switch endpoint.(type) {
	case testConfig:
		return true
	default:
		return false
	}
}

func (t testEventSender) SendEvent(ce cloudevents.Event, endpoint interface{}) error {
	cfg := endpoint.(testConfig)
	if cfg.valid {
		return nil
	}
	return fmt.Errorf("can't send %#v to %s server at %s topic",
		ce, cfg.servers, cfg.topic)
}
