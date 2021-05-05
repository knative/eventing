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
	"testing"
	"time"

	"github.com/prometheus/common/log"
	"github.com/wavesoftware/go-ensure"
	"go.uber.org/zap"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

var (
	// Interval is used to send events in specific rate.
	Interval = 10 * time.Millisecond
)

// Prober is the interface for a prober, which checks the result of the probes when stopped.
type Prober interface {
	// Verify will verify prober state after finished has been send
	Verify() ([]error, int)

	// Finish send finished event
	Finish()

	// ReportErrors will reports found errors in proper way
	ReportErrors(errors []error)
}

// ProbeRunner will run continual verification with provided configuration.
type ProbeRunner interface {
	// Setup will start a continual prober in background.
	Setup(ctx pkgupgrade.Context)

	// Verify will verify that all sent events propagated at least once.
	Verify(ctx pkgupgrade.Context)
}

// CreateProbeRunner will create a runner compatible with
// pkgupgrade.BackgroundVerification interface.
func CreateProbeRunner(
	config *Config,
	options ...testlib.SetupClientOption,
) ProbeRunner {
	return &probeRunner{
		prober: &prober{
			config: config,
		},
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
	defer testlib.TearDown(p.client)
	defer p.remove()
	p.Finish()
	waitAfterFinished(p.prober)

	errors, events := p.prober.Verify()
	if len(errors) == 0 {
		t.Logf("All %d events propagated well", events)
	} else {
		t.Logf("There were %d events propagated, but %d errors occurred. "+
			"Listing them below.", events, len(errors))
	}

	p.ReportErrors(errors)
}

// RunEventProber starts a single Prober of the given domain.
// Deprecated: use CreateProbeRunner func instead.
func RunEventProber(ctx context.Context, log *zap.SugaredLogger, client *testlib.Client, config *Config) Prober {
	log.Warn("prober.RunEventProber is deprecated. Use CreateProbeRunner instead.")
	config.Ctx = ctx
	p := &prober{
		log:    log,
		client: client,
		config: config,
	}
	p.deploy()
	return p
}

// AssertEventProber will send finish event and then verify if all events propagated well
// Deprecated: use CreateProbeRunner func instead.
func AssertEventProber(ctx context.Context, t *testing.T, probe Prober) {
	log.Warn("prober.AssertEventProber is deprecated. Use CreateProbeRunner instead.")
	p := probe.(*prober)
	p.client.T = t
	p.config.Ctx = ctx
	pr := &probeRunner{
		prober:  p,
		options: nil,
	}
	pr.Verify(pkgupgrade.Context{
		T:   t,
		Log: p.log,
	})
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
	ensure.NoError(testlib.AwaitForAll(p.log))
	// allow sender to send at least some events, 2 sec wait
	time.Sleep(2 * time.Second)
	p.log.Infof("Prober is now sending events with interval of %v in "+
		"namespace: %v", p.config.Interval, p.client.Namespace)
}

func (p *prober) remove() {
	if p.config.Serving.Use {
		p.removeForwarder()
	}
	ensure.NoError(p.client.Tracker.Clean(true))
}

func waitAfterFinished(p *prober) {
	p.log.Infof("Waiting %v after sender finished...", p.config.FinishedSleep)
	time.Sleep(p.config.FinishedSleep)
}
