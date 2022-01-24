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
	"time"

	"go.uber.org/zap"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

var (
	// Interval is used to send events in specific rate.
	Interval = 10 * time.Millisecond
)

// Runner will run continual verification with provided configuration.
type Runner interface {
	// Setup will start a continual prober in background.
	Setup(ctx pkgupgrade.Context)

	// Verify will verify that all sent events propagated at least once.
	Verify(ctx pkgupgrade.Context)
}

// NewRunner will create a runner compatible with NewContinualVerification
// func.
func NewRunner(config *Config, options ...testlib.SetupClientOption) Runner {
	return &probeRunner{
		prober:  &prober{config: config},
		options: options,
	}
}

type probeRunner struct {
	*prober
	options []testlib.SetupClientOption
}

func (p *probeRunner) Setup(ctx pkgupgrade.Context) {
	p.log = ctx.Log
	p.client = testlib.Setup(ctx.T, false, p.options...)
	p.deploy()
}

func (p *probeRunner) Verify(ctx pkgupgrade.Context) {
	if p.client == nil {
		ctx.T.Fatal("prober isn't initiated (client is nil)")
		return
	}
	// use T from new test
	p.client.T = ctx.T
	t := ctx.T
	p.Finish()

	errors, events := p.prober.Verify()
	if len(errors) == 0 {
		t.Logf("All %d events propagated well", events)
	} else {
		t.Logf("There were %d events propagated, but %d errors occurred. "+
			"Listing them below.", events, len(errors))
	}

	p.ReportErrors(errors)
	if !ctx.T.Failed() {
		p.remove()
		testlib.TearDown(p.client)
	}
}

type prober struct {
	log    *zap.SugaredLogger
	client *testlib.Client
	config *Config
}

func (p *prober) servingClient() resources.ServingClient {
	return resources.ServingClient{
		Kube:    p.client.Kube,
		Dynamic: p.client.Dynamic,
	}
}

func (p *prober) ReportErrors(errors []error) {
	t := p.client.T
	for _, err := range errors {
		if p.config.FailOnErrors {
			t.Error(err)
		} else {
			p.log.Warnf("Silenced FAIL: %v", err)
		}
	}
	if len(errors) > 0 && !p.config.FailOnErrors {
		t.Skipf(
			"Found %d errors, but FailOnErrors is false. Skipping test.",
			len(errors),
		)
	}
}

func (p *prober) deploy() {
	p.log.Infof("Using namespace for probe testing: %v", p.client.Namespace)
	p.deployConfiguration()
	p.deployReceiver()
	if p.config.Serving.Use {
		p.deployForwarder()
	}
	p.client.WaitForAllTestResourcesReadyOrFail(p.config.Ctx)

	p.deploySender()
	p.ensureNoError(testlib.AwaitForAll(p.log))
	// allow sender to send at least some events, 2 sec wait
	time.Sleep(2 * time.Second)
	p.log.Infof("Prober is now sending events with interval of %v in "+
		"namespace: %v", p.config.Interval, p.client.Namespace)
}

func (p *prober) remove() {
	if p.config.Serving.Use {
		p.removeForwarder()
	}
	p.ensureNoError(p.client.Tracker.Clean(true))
}
