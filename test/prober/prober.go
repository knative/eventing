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

var (
	Version = "v0.6.0"
)

// Prober is the interface for a prober, which checks the result of the probes when stopped.
type Prober interface {
	// Verify will verify prober state after finished has been send
	Verify() []error

	// Finish send finished event
	Finish()

	// deploy a prober to a cluster
	deploy()
	// remove a prober from cluster
	remove()
}

// Config represents a configuration for prober
type Config struct {
	Namespace     string
	Interval      time.Duration
	UseServing    bool
	FinishedSleep time.Duration
}

func NewConfig(namespace string) *Config {
	return &Config{
		Namespace:     namespace,
		Interval:      10 * time.Millisecond,
		UseServing:    true,
		FinishedSleep: 5 * time.Second,
	}
}

// RunEventProber starts a single Prober of the given domain.
func RunEventProber(logf logging.FormatLogger, client *common.Client, config *Config) Prober {
	pm := newProber(logf, client, config)
	pm.deploy()
	return pm
}

// AssertEventProber will send finish event and then verify if all events propagated well
func AssertEventProber(t *testing.T, prober Prober) {
	prober.Finish()

	waitAfterFinished(prober)

	errors := prober.Verify()
	for _, err := range errors {
		t.Error(err)
	}
	if len(errors) == 0 {
		t.Log("All events propagated well")
	}

	prober.remove()
}

type prober struct {
	logf   logging.FormatLogger
	client *common.Client
	config *Config
}

func (p *prober) Verify() []error {
	p.logf("ERR: Verify(): implement me")
	return make([]error, 0)
}

func (p *prober) Finish() {
	p.removeSender()
}

func (p *prober) deploy() {
	p.deployConfiguration()
	p.deployReceiver()
	if p.config.UseServing {
		p.deployForwarder()
	}
	p.deploySender()
}

func (p *prober) remove() {
	if p.config.UseServing {
		p.removeForwarder()
	}
	p.removeReceiver()
	p.removeConfiguration()
}

func newProber(logf logging.FormatLogger, client *common.Client, config *Config) Prober {
	return &prober{
		logf:   logf,
		client: client,
		config: config,
	}
}

func waitAfterFinished(p Prober) {
	s := p.(*prober)
	cfg := s.config
	s.logf("Waiting %v after sender finished...", cfg.FinishedSleep)
	time.Sleep(cfg.FinishedSleep)
}
