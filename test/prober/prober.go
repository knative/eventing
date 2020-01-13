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

package prober

import (
	"knative.dev/eventing/test/common"
	"knative.dev/pkg/test/logging"
	"testing"
	"time"
)

// Prober is the interface for a prober, which checks the result of the probes when stopped.
type Prober interface {
	// Verify will verify prober state after finished has been send
	Verify() []error

	// Finish send finished event
	Finish()

	// deploy a prober to a cluster
	deploy()
	// undeploy a prober from cluster
	undeploy()
}

// ProberConfig represents a configuration for prober
type ProberConfig struct {
	Namespace  string
	Interval   time.Duration
	UseServing bool
}

// RunEventProber starts a single Prober of the given domain.
func RunEventProber(logf logging.FormatLogger, client *common.Client, config ProberConfig) Prober {
	pm := newProber(logf, client, config)
	pm.deploy()
	return pm
}

// AssertEventProber will send finish event and then verify if all events propagated well
func AssertEventProber(t *testing.T, prober Prober) {
	prober.Finish()

	errors := prober.Verify()
	for _, err := range errors {
		t.Error(err)
	}
	if len(errors) == 0 {
		t.Log("All events propagated well")
	}

	prober.undeploy()
}

type prober struct {
	logf   logging.FormatLogger
	client *common.Client
	config ProberConfig
}

func (p *prober) Verify() []error {
	p.logf("ERR: Verify(): implement me")
	return make([]error, 0)
}

func (p *prober) Finish() {
	p.logf("ERR: Finish(): implement me")
}

func (p *prober) deploy() {
	p.logf("ERR: deploy(): implement me")

}

func (p *prober) undeploy() {
	p.logf("ERR: undeploy(): implement me")
}

func newProber(logf logging.FormatLogger, client *common.Client, config ProberConfig) Prober {
	return &prober{
		logf:   logf,
		client: client,
		config: config,
	}
}