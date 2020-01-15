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
	"go.uber.org/zap"
	"knative.dev/eventing/test/common"
	"testing"
	"time"
)

var (
	Version  = "v0.6.0"
	// FIXME: Interval is set to 200 msec, as lower values will result in errors
	// https://github.com/knative/eventing/issues/2357
	// Interval = 10 * time.Millisecond
	Interval = 200 * time.Millisecond
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
	Serving       ServingConfig
	FinishedSleep time.Duration
}

type ServingConfig struct {
	Use         bool
	ScaleToZero bool
}

func NewConfig(namespace string) *Config {
	return &Config{
		Namespace:     namespace,
		Interval:      Interval,
		FinishedSleep: 5 * time.Second,
		Serving: ServingConfig{
			Use:         true,
			ScaleToZero: true,
		},
	}
}

// RunEventProber starts a single Prober of the given domain.
func RunEventProber(log *zap.SugaredLogger, client *common.Client, config *Config) Prober {
	pm := newProber(log, client, config)
	pm.deploy()
	return pm
}

// AssertEventProber will send finish event and then verify if all events propagated well
func AssertEventProber(t *testing.T, prober Prober) {
	prober.Finish()

	waitAfterFinished(prober)

	errors := prober.Verify()
	if len(errors) == 0 {
		t.Log("All events propagated well")
	} else {
		t.Logf("There ware %v errors. Listing tem below.", len(errors))
	}
	for _, err := range errors {
		t.Error(err)
	}

	prober.remove()
}

type prober struct {
	log    *zap.SugaredLogger
	client *common.Client
	config *Config
}

func (p *prober) Verify() []error {
	p.log.Error("Verify(): implement me")
	return make([]error, 0)
}

func (p *prober) Finish() {
	p.removeSender()
}

func (p *prober) deploy() {
	p.deployConfiguration()
	p.deployReceiver()
	if p.config.Serving.Use {
		p.deployForwarder()
	}
	awaitAll(p.log)

	p.deploySender()
	awaitAll(p.log)
}

func (p *prober) remove() {
	if p.config.Serving.Use {
		p.removeForwarder()
	}
	p.removeReceiver()
	p.removeConfiguration()
}

func newProber(log *zap.SugaredLogger, client *common.Client, config *Config) Prober {
	return &prober{
		log:    log,
		client: client,
		config: config,
	}
}

func waitAfterFinished(p Prober) {
	s := p.(*prober)
	cfg := s.config
	s.log.Infof("Waiting %v after sender finished...", cfg.FinishedSleep)
	time.Sleep(cfg.FinishedSleep)
}
