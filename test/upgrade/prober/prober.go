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
	"context"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wavesoftware/go-ensure"
	"go.uber.org/zap"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
)

var (
	// FIXME: Interval is set to 200 msec, as lower values will result in errors: knative/eventing#2357
	// Interval = 10 * time.Millisecond
	Interval = 200 * time.Millisecond
)

// Prober is the interface for a prober, which checks the result of the probes when stopped.
type Prober struct {
	log    *zap.SugaredLogger
	client *testlib.Client
	config *Config
}

type ImagesConfig struct {
	Sender    string
	Receiver  string
	Forwarder string
}

// Config represents a configuration for prober
type Config struct {
	Namespace     string
	Interval      time.Duration
	Serving       ServingConfig
	FinishedSleep time.Duration
	FailOnErrors  bool
	Images        ImagesConfig
}

type ServingConfig struct {
	Use         bool
	ScaleToZero bool
}

func NewConfig(namespace string) *Config {
	config := &Config{
		Namespace:     "",
		Interval:      Interval,
		FinishedSleep: 5 * time.Second,
		FailOnErrors:  true,
		Serving: ServingConfig{
			Use:         false,
			ScaleToZero: true,
		},
	}
	err := envconfig.Process("e2e_upgrade_tests", config)
	ensure.NoError(err)
	config.Namespace = namespace
	return config
}

// NewProber returns a new Prober of the given domain.
func NewProber(log *zap.SugaredLogger, client *testlib.Client, config *Config) Prober {
	return Prober{
		log:    log,
		client: client,
		config: config,
	}
}

func (p *Prober) servingClient() resources.ServingClient {
	return resources.ServingClient{
		Kube:    p.client.Kube,
		Dynamic: p.client.Dynamic,
	}
}

func (p *Prober) Deploy(ctx context.Context) {
	p.log.Infof("Using namespace for probe testing: %v", p.client.Namespace)
	p.deployConfiguration()
	p.deployReceiver(ctx)
	if p.config.Serving.Use {
		p.deployForwarder(ctx)
	}
	p.client.WaitForAllTestResourcesReadyOrFail(ctx)

	p.deploySender(ctx)
	ensure.NoError(testlib.AwaitForAll(p.log))
	// allow sender to send at least some events, 2 sec wait
	time.Sleep(2 * time.Second)
	p.log.Infof("Prober is now sending events with interval of %v in "+
		"namespace: %v", p.config.Interval, p.client.Namespace)
}

func (p *Prober) Remove() {
	if p.config.Serving.Use {
		p.removeForwarder()
	}
	ensure.NoError(p.client.Tracker.Clean(true))
}
