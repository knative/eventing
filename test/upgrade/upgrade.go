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
	"log"
	"os"
	"sync"
	"testing"

	"knative.dev/eventing/test/rekt/features/channel"
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

var mux = &sync.Mutex{}

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
// the feature. The "verify" phase should only verify its function. It should be possible to
// call this function multiple times and still properly verify the feature (e.g. one call
// after upgrade, one call after downgrade).
type DurableFeature struct {
	SetupF    *feature.Feature
	VerifyF   *feature.Feature
	Namespace string
	Global    environment.GlobalEnvironment
}

func (fe *DurableFeature) Setup(label string) pkgupgrade.Operation {
	return pkgupgrade.NewOperation(label, func(c pkgupgrade.Context) {
		c.T.Parallel()
		ctx, env := fe.Global.Environment(
			knative.WithKnativeNamespace(system.Namespace()),
			knative.WithLoggingConfig,
			knative.WithTracingConfig,
			k8s.WithEventListener,
			// Not managed - namespace preserved.
		)
		fe.Namespace = env.Namespace()
		env.Test(ctx, c.T, fe.SetupF)
	})
}

func (fe *DurableFeature) Verify(label string) pkgupgrade.Operation {
	return pkgupgrade.NewOperation(label, func(c pkgupgrade.Context) {
		c.T.Parallel()
		ctx, env := fe.Global.Environment(
			knative.WithKnativeNamespace(system.Namespace()),
			knative.WithLoggingConfig,
			knative.WithTracingConfig,
			k8s.WithEventListener,
			// Re-use namespace.
			environment.InNamespace(fe.Namespace),
			// Not managed - namespace preserved.
		)
		env.Test(ctx, c.T, fe.VerifyF)
	})
}

func (fe *DurableFeature) VerifyTeardown(label string) pkgupgrade.Operation {
	return pkgupgrade.NewOperation(label, func(c pkgupgrade.Context) {
		c.T.Parallel()
		ctx, env := fe.Global.Environment(
			knative.WithKnativeNamespace(system.Namespace()),
			knative.WithLoggingConfig,
			knative.WithTracingConfig,
			k8s.WithEventListener,
			environment.InNamespace(fe.Namespace),
			// Ensures teardown of resources/namespace.
			environment.Managed(c.T),
		)
		env.Test(ctx, c.T, fe.VerifyF)
	})
}

func (fe *DurableFeature) SetupVerifyTeardown(label string) pkgupgrade.Operation {
	return pkgupgrade.NewOperation(label, func(c pkgupgrade.Context) {
		c.T.Parallel()
		ctx, env := fe.Global.Environment(
			knative.WithKnativeNamespace(system.Namespace()),
			knative.WithLoggingConfig,
			knative.WithTracingConfig,
			k8s.WithEventListener,
			// Ensures teardown of resources/namespace.
			environment.Managed(c.T),
		)
		env.Test(ctx, c.T, fe.SetupF)
		env.Test(ctx, c.T, fe.VerifyF)
	})
}

type FeatureWithUpgradeTests interface {
	PreUpgradeTests() []pkgupgrade.Operation
	PostUpgradeTests() []pkgupgrade.Operation
	PostDowngradeTests() []pkgupgrade.Operation
}

// NewFeatureGroupOnlyUpgrade creates a new feature group with these actions:
// Pre-upgrade: Setup, Assert
// Post-upgrade: Assert, Teardown
// Post-downgrade: no-op.
func NewFeatureGroupOnlyUpgrade(fg *DurableFeatureGroup) featureGroupWithPostUpgradeTeardown {
	return featureGroupWithPostUpgradeTeardown{
		label: "OnlyUpgrade",
		group: fg,
	}
}

type featureGroupWithPostUpgradeTeardown struct {
	label string
	group *DurableFeatureGroup
}

func (f *featureGroupWithPostUpgradeTeardown) PreUpgradeTests() []pkgupgrade.Operation {
	return f.group.Setup(f.label)
}

func (f *featureGroupWithPostUpgradeTeardown) PostUpgradeTests() []pkgupgrade.Operation {
	return f.group.VerifyTeardown(f.label)
}

func (f *featureGroupWithPostUpgradeTeardown) PostDowngradeTests() []pkgupgrade.Operation {
	// No-op. Teardown was done post-upgrade.
	return nil
}

// NewFeatureGroupUpgradeDowngrade creates a new feature group with these actions:
// Pre-upgrade: Setup, Verify.
// Post-upgrade: Verify.
// Post-downgrade: Verify, Teardown.
func NewFeatureGroupUpgradeDowngrade(fg *DurableFeatureGroup) featureGroupWithPostDowngradeTeardown {
	return featureGroupWithPostDowngradeTeardown{
		label: "BothUpgradeDowngrade",
		group: fg,
	}
}

type featureGroupWithPostDowngradeTeardown struct {
	label string
	group *DurableFeatureGroup
}

func (f *featureGroupWithPostDowngradeTeardown) PreUpgradeTests() []pkgupgrade.Operation {
	return f.group.Setup(f.label)
}

func (f *featureGroupWithPostDowngradeTeardown) PostUpgradeTests() []pkgupgrade.Operation {
	// PostUpgrade only asserts existing resources. Teardown will be done post-downgrade.
	return f.group.Verify(f.label)
}

func (f *featureGroupWithPostDowngradeTeardown) PostDowngradeTests() []pkgupgrade.Operation {
	return f.group.VerifyTeardown(f.label)
}

// NewFeatureGroupOnlyDowngrade creates a new feature group with these actions:
// Pre-upgrade: no-op.
// Post-upgrade: Setup, Verify.
// Post-downgrade: Verify, Teardown.
func NewFeatureGroupOnlyDowngrade(fg *DurableFeatureGroup) featureGroupWithPostUpgradeSetup {
	return featureGroupWithPostUpgradeSetup{
		label: "OnlyDowngrade",
		group: fg,
	}
}

type featureGroupWithPostUpgradeSetup struct {
	label string
	group *DurableFeatureGroup
}

func (f *featureGroupWithPostUpgradeSetup) PreUpgradeTests() []pkgupgrade.Operation {
	// No-op. Resources will be created post-upgrade.
	return nil
}

func (f *featureGroupWithPostUpgradeSetup) PostUpgradeTests() []pkgupgrade.Operation {
	// Resources created post-upgrade.
	return f.group.Setup(f.label)
}

func (f *featureGroupWithPostUpgradeSetup) PostDowngradeTests() []pkgupgrade.Operation {
	// Assert and Teardown is done post-downgrade.
	return f.group.VerifyTeardown(f.label)
}

// NewFeatureGroupSmoke creates a new feature group with these actions:
// Pre-upgrade: no-op.
// Post-upgrade: Setup, Verify, Teardown.
// Post-downgrade: Setup, Verify, Teardown.
func NewFeatureGroupSmoke(fg *DurableFeatureGroup) featureGroupSmoke {
	return featureGroupSmoke{
		label: "Smoke",
		group: fg,
	}
}

type featureGroupSmoke struct {
	label string
	group *DurableFeatureGroup
}

func (f *featureGroupSmoke) PreUpgradeTests() []pkgupgrade.Operation {
	// No-op. No need to smoke test before upgrade.
	return nil
}

func (f *featureGroupSmoke) PostUpgradeTests() []pkgupgrade.Operation {
	return f.group.SetupVerifyTeardown(f.label)
}

func (f *featureGroupSmoke) PostDowngradeTests() []pkgupgrade.Operation {
	return f.group.SetupVerifyTeardown(f.label)
}

type DurableFeatureGroup []*DurableFeature

func (fg DurableFeatureGroup) Setup(label string) []pkgupgrade.Operation {
	ops := make([]pkgupgrade.Operation, 0, len(fg))
	for _, ft := range fg {
		ops = append(ops, ft.Setup(label))
	}
	return ops
}

func (fg DurableFeatureGroup) Verify(label string) []pkgupgrade.Operation {
	ops := make([]pkgupgrade.Operation, 0, len(fg))
	for _, ft := range fg {
		ops = append(ops, ft.Verify(label))
	}
	return ops
}

func (fg DurableFeatureGroup) VerifyTeardown(label string) []pkgupgrade.Operation {
	ops := make([]pkgupgrade.Operation, 0, len(fg))
	for _, ft := range fg {
		ops = append(ops, ft.VerifyTeardown(label))
	}
	return ops
}

func (fg DurableFeatureGroup) SetupVerifyTeardown(label string) []pkgupgrade.Operation {
	ops := make([]pkgupgrade.Operation, 0, len(fg))
	for _, ft := range fg {
		ops = append(ops, ft.SetupVerifyTeardown(label))
	}
	return ops
}

func InMemoryChannelFeature(glob environment.GlobalEnvironment) *DurableFeature {
	// Prevent race conditions on channel_impl.EnvCfg.ChannelGK when running tests in parallel.
	mux.Lock()
	defer mux.Unlock()
	channel_impl.EnvCfg.ChannelGK = "InMemoryChannel.messaging.knative.dev"
	channel_impl.EnvCfg.ChannelV = "v1"

	createSubscriberFn := func(ref *duckv1.KReference, uri string) manifest.CfgFn {
		return subscription.WithSubscriber(ref, uri, "")
	}

	setupF := feature.NewFeature()
	sink, ch := channel.ChannelChainSetup(setupF, 1, createSubscriberFn)

	verifyF := feature.NewFeature()
	channel.ChannelChainAssert(verifyF, sink, ch)

	return &DurableFeature{SetupF: setupF, VerifyF: verifyF, Global: glob}
}
