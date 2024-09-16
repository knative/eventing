/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package upgrade

import (
	"context"
	"log"
	"os"
	"sync"
	"testing"

	"knative.dev/eventing/pkg/apis/eventing"
	brokerfeatures "knative.dev/eventing/test/rekt/features/broker"
	"knative.dev/eventing/test/rekt/features/channel"
	brokerresources "knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/subscription"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"
	pkgupgrade "knative.dev/pkg/test/upgrade"
	"knative.dev/pkg/test/zipkin"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
)

var (
	channelConfigMux = &sync.Mutex{}
	brokerConfigMux  = &sync.Mutex{}
	opts             = []environment.EnvOpts{
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
	}
)

// RunMainTest expects flags to be already initialized.
// This function needs to be exposed, so that test cases in other repositories can call the upgrade
// main tests in eventing.
func RunMainTest(m *testing.M) {
	os.Exit(func() int {
		// Any tests may SetupZipkinTracing, it will only actually be done once. This should be the ONLY
		// place that cleans it up. If an individual test calls this instead, then it will break other
		// tests that need the tracing in place.
		defer zipkin.CleanupZipkinTracingSetup(log.Printf)
		return m.Run()
	}())
}

// DurableFeature holds the setup and verify phase of a feature. The "setup" phase should set up
// the feature. The "verify" phase should only verify its function. This function should be
// idempotent. Calling this function multiple times should still properly verify the feature
// (e.g. one call after upgrade, one call after downgrade).
type DurableFeature struct {
	SetupF *feature.Feature
	// EnvOpts should never include environment.Managed or environment.Cleanup as these functions
	// break the functionality.
	EnvOpts  []environment.EnvOpts
	setupEnv environment.Environment
	setupCtx context.Context
	VerifyF  func() *feature.Feature
	Global   environment.GlobalEnvironment
}

func (fe *DurableFeature) Setup(label string) pkgupgrade.Operation {
	return pkgupgrade.NewOperation(label, func(c pkgupgrade.Context) {
		c.T.Parallel()
		ctx, env := fe.Global.Environment(
			fe.EnvOpts...,
		// Not managed - namespace preserved.
		)
		fe.setupEnv = env
		fe.setupCtx = ctx
		env.Test(ctx, c.T, fe.SetupF)
	})
}

func (fe *DurableFeature) Verify(label string) pkgupgrade.Operation {
	return pkgupgrade.NewOperation(label, func(c pkgupgrade.Context) {
		c.T.Parallel()
		fe.setupEnv.Test(fe.setupCtx, c.T, fe.VerifyF())
	})
}

func (fe *DurableFeature) VerifyAndTeardown(label string) pkgupgrade.Operation {
	return pkgupgrade.NewOperation(label, func(c pkgupgrade.Context) {
		c.T.Parallel()
		fe.setupEnv.Test(fe.setupCtx, c.T, fe.VerifyF())
		// Ensures teardown of resources/namespace.
		fe.setupEnv.Finish()
	})
}

func (fe *DurableFeature) SetupVerifyAndTeardown(label string) pkgupgrade.Operation {
	return pkgupgrade.NewOperation(label, func(c pkgupgrade.Context) {
		c.T.Parallel()
		ctx, env := fe.Global.Environment(
			append(fe.EnvOpts, environment.Managed(c.T))...,
		)
		env.Test(ctx, c.T, fe.SetupF)
		env.Test(ctx, c.T, fe.VerifyF())
	})
}

type FeatureWithUpgradeTests interface {
	PreUpgradeTests() []pkgupgrade.Operation
	PostUpgradeTests() []pkgupgrade.Operation
	PostDowngradeTests() []pkgupgrade.Operation
}

// NewFeatureOnlyUpgrade decorates a feature with these actions:
// Pre-upgrade: Setup, Verify
// Post-upgrade: Verify, Teardown
// Post-downgrade: no-op.
func NewFeatureOnlyUpgrade(f *DurableFeature) FeatureWithUpgradeTests {
	return featureOnlyUpgrade{
		label:   "OnlyUpgrade",
		feature: f,
	}
}

type featureOnlyUpgrade struct {
	label   string
	feature *DurableFeature
}

func (f featureOnlyUpgrade) PreUpgradeTests() []pkgupgrade.Operation {
	return []pkgupgrade.Operation{
		f.feature.Setup(f.label),
	}
}

func (f featureOnlyUpgrade) PostUpgradeTests() []pkgupgrade.Operation {
	return []pkgupgrade.Operation{
		f.feature.VerifyAndTeardown(f.label),
	}
}

func (f featureOnlyUpgrade) PostDowngradeTests() []pkgupgrade.Operation {
	// No-op. Teardown was done post-upgrade.
	return nil
}

// NewFeatureUpgradeDowngrade decorates a feature with these actions:
// Pre-upgrade: Setup, Verify.
// Post-upgrade: Verify.
// Post-downgrade: Verify, Teardown.
func NewFeatureUpgradeDowngrade(f *DurableFeature) FeatureWithUpgradeTests {
	return featureUpgradeDowngrade{
		label:   "BothUpgradeDowngrade",
		feature: f,
	}
}

type featureUpgradeDowngrade struct {
	label   string
	feature *DurableFeature
}

func (f featureUpgradeDowngrade) PreUpgradeTests() []pkgupgrade.Operation {
	return []pkgupgrade.Operation{
		f.feature.Setup(f.label),
	}
}

func (f featureUpgradeDowngrade) PostUpgradeTests() []pkgupgrade.Operation {
	// PostUpgrade only asserts existing resources. Teardown will be done post-downgrade.
	return []pkgupgrade.Operation{
		f.feature.Verify(f.label),
	}
}

func (f featureUpgradeDowngrade) PostDowngradeTests() []pkgupgrade.Operation {
	return []pkgupgrade.Operation{
		f.feature.VerifyAndTeardown(f.label),
	}
}

// NewFeatureOnlyDowngrade decorates a feature with these actions:
// Pre-upgrade: no-op.
// Post-upgrade: Setup, Verify.
// Post-downgrade: Verify, Teardown.
func NewFeatureOnlyDowngrade(f *DurableFeature) FeatureWithUpgradeTests {
	return featureOnlyDowngrade{
		label:   "OnlyDowngrade",
		feature: f,
	}
}

type featureOnlyDowngrade struct {
	label   string
	feature *DurableFeature
}

func (f featureOnlyDowngrade) PreUpgradeTests() []pkgupgrade.Operation {
	// No-op. Resources will be created post-upgrade.
	return nil
}

func (f featureOnlyDowngrade) PostUpgradeTests() []pkgupgrade.Operation {
	// Resources created post-upgrade.
	return []pkgupgrade.Operation{
		f.feature.Setup(f.label),
	}
}

func (f featureOnlyDowngrade) PostDowngradeTests() []pkgupgrade.Operation {
	// Assert and Teardown is done post-downgrade.
	return []pkgupgrade.Operation{
		f.feature.VerifyAndTeardown(f.label),
	}
}

// NewFeatureSmoke decorates a feature with these actions:
// Pre-upgrade: no-op.
// Post-upgrade: Setup, Verify, Teardown.
// Post-downgrade: Setup, Verify, Teardown.
func NewFeatureSmoke(f *DurableFeature) FeatureWithUpgradeTests {
	return featureSmoke{
		label:   "Smoke",
		feature: f,
	}
}

type featureSmoke struct {
	label   string
	feature *DurableFeature
}

func (f featureSmoke) PreUpgradeTests() []pkgupgrade.Operation {
	// No-op. No need to smoke test before upgrade.
	return nil
}

func (f featureSmoke) PostUpgradeTests() []pkgupgrade.Operation {
	return []pkgupgrade.Operation{
		f.feature.SetupVerifyAndTeardown(f.label),
	}
}

func (f featureSmoke) PostDowngradeTests() []pkgupgrade.Operation {
	return []pkgupgrade.Operation{
		f.feature.SetupVerifyAndTeardown(f.label),
	}
}

// FeatureGroupWithUpgradeTests aggregates tests across a group of features.
type FeatureGroupWithUpgradeTests []FeatureWithUpgradeTests

func (fg FeatureGroupWithUpgradeTests) PreUpgradeTests() []pkgupgrade.Operation {
	ops := make([]pkgupgrade.Operation, 0, len(fg))
	for _, ft := range fg {
		ops = append(ops, ft.PreUpgradeTests()...)
	}
	return ops
}

func (fg FeatureGroupWithUpgradeTests) PostUpgradeTests() []pkgupgrade.Operation {
	ops := make([]pkgupgrade.Operation, 0, len(fg))
	for _, ft := range fg {
		ops = append(ops, ft.PostUpgradeTests()...)
	}
	return ops
}

func (fg FeatureGroupWithUpgradeTests) PostDowngradeTests() []pkgupgrade.Operation {
	ops := make([]pkgupgrade.Operation, 0, len(fg))
	for _, ft := range fg {
		ops = append(ops, ft.PostDowngradeTests()...)
	}
	return ops
}

func InMemoryChannelFeature(glob environment.GlobalEnvironment) *DurableFeature {
	// Prevent race conditions on channel_impl.EnvCfg.ChannelGK when running tests in parallel.
	channelConfigMux.Lock()
	defer channelConfigMux.Unlock()
	channel_impl.EnvCfg.ChannelGK = "InMemoryChannel.messaging.knative.dev"
	channel_impl.EnvCfg.ChannelV = "v1"

	createSubscriberFn := func(ref *duckv1.KReference, uri string) manifest.CfgFn {
		return subscription.WithSubscriber(ref, uri, "")
	}

	setupF := feature.NewFeature()
	sink, ch := channel.ChannelChainSetup(setupF, 1, createSubscriberFn)

	verifyF := func() *feature.Feature {
		f := feature.NewFeatureNamed(setupF.Name)
		channel.ChannelChainAssert(f, sink, ch)
		return f
	}

	return &DurableFeature{SetupF: setupF, VerifyF: verifyF, Global: glob, EnvOpts: opts}
}

func BrokerEventTransformationForTrigger(glob environment.GlobalEnvironment,
) *DurableFeature {
	// Prevent race conditions on EnvCfg.BrokerClass when running tests in parallel.
	brokerConfigMux.Lock()
	defer brokerConfigMux.Unlock()
	brokerresources.EnvCfg.BrokerClass = eventing.MTChannelBrokerClassValue

	setupF := feature.NewFeature()
	cfg := brokerfeatures.BrokerEventTransformationForTriggerSetup(setupF)

	verifyF := func() *feature.Feature {
		f := feature.NewFeatureNamed(setupF.Name)
		brokerfeatures.BrokerEventTransformationForTriggerAssert(f, cfg)
		return f
	}

	return &DurableFeature{SetupF: setupF, VerifyF: verifyF, Global: glob, EnvOpts: opts}
}
