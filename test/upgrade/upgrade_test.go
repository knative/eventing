//go:build upgrade
// +build upgrade

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
	"flag"
	"log"
	"testing"

	"knative.dev/eventing/test"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/upgrade/installation"
	"knative.dev/pkg/system"
	pkgtest "knative.dev/pkg/test"
	pkgupgrade "knative.dev/pkg/test/upgrade"
	"knative.dev/reconciler-test/pkg/environment"
)

var global environment.GlobalEnvironment

func TestEventingUpgrades(t *testing.T) {
	labels := []string{
		"eventing-controller",
		"eventing-webhook",
		"imc-controller",
		"imc-dispatcher",
		"mt-broker-controller",
		"mt-broker-ingress",
		"mt-broker-filter",
	}
	canceler := testlib.ExportLogStreamOnError(t, testlib.SystemLogsDir, system.Namespace(), labels...)
	defer canceler()

	commonFeatureGroup := &DurableFeatureGroup{
		InMemoryChannelFeature(global),
	}
	// Feature group that will run the same test post-upgrade and post-downgrade
	// creating new resource every time.
	// Pre-upgrade: no-op.
	// Post-upgrade: Setup, Verify, Teardown
	// Post-downgrade: Setup, Verify, Teardown
	featSmoke := NewFeatureGroupSmoke(commonFeatureGroup)
	// Feature group that will be created pre-upgrade and verified/removed post-upgrade.
	// Pre-upgrade: Setup, Verify
	// Post-upgrade: Verify, Teardown
	// Post-downgrade: no-op.
	featOnlyUpgrade := NewFeatureGroupOnlyUpgrade(commonFeatureGroup)
	// Feature group that will be created pre-upgrade and verified post-upgrade, verified and removed post-downgrade
	// Pre-upgrade: Setup, Verify.
	// Post-upgrade: Verify.
	// Post-downgrade: Verify, Teardown.
	featBothUpgradeDowngrade := NewFeatureGroupUpgradeDowngrade(commonFeatureGroup)
	// Feature group that will be created post-upgrade, verified and removed post-downgrade.
	// Pre-upgrade: no-op.
	// Post-upgrade: Setup, Verify.
	// Post-downgrade: Verify, Teardown.
	featOnlyDowngrade := NewFeatureGroupOnlyDowngrade(commonFeatureGroup)

	suite := pkgupgrade.Suite{
		Tests: pkgupgrade.Tests{
			PreUpgrade: merge(
				featSmoke.PreUpgradeTests(),
				featOnlyUpgrade.PreUpgradeTests(),
				featBothUpgradeDowngrade.PreUpgradeTests(),
				featOnlyDowngrade.PreUpgradeTests(),
			),
			PostUpgrade: merge(
				[]pkgupgrade.Operation{
					CRDPostUpgradeTest(),
				},
				featSmoke.PostUpgradeTests(),
				featOnlyUpgrade.PostUpgradeTests(),
				featBothUpgradeDowngrade.PostUpgradeTests(),
				featOnlyDowngrade.PostUpgradeTests(),
			),
			PostDowngrade: merge(
				featSmoke.PostDowngradeTests(),
				featOnlyUpgrade.PostDowngradeTests(),
				featBothUpgradeDowngrade.PostDowngradeTests(),
				featOnlyDowngrade.PostDowngradeTests(),
			),
			Continual: []pkgupgrade.BackgroundOperation{
				ContinualTest(),
			},
		},
		Installations: pkgupgrade.Installations{
			Base: []pkgupgrade.Operation{
				installation.LatestStable(),
			},
			UpgradeWith: []pkgupgrade.Operation{
				installation.GitHead(),
			},
			DowngradeWith: []pkgupgrade.Operation{
				installation.LatestStable(),
			},
		},
	}
	suite.Execute(pkgupgrade.Configuration{T: t})
}

func TestMain(m *testing.M) {
	test.InitializeEventingFlags()

	restConfig, err := pkgtest.Flags.ClientConfig.GetRESTConfig()
	if err != nil {
		log.Fatal("Error building client config: ", err)
	}

	// Getting the rest config explicitly and passing it further will prevent re-initializing the flagset
	// in NewStandardGlobalEnvironment(). The upgrade tests use knative.dev/pkg/test which initializes the
	// flagset as well.
	global = environment.NewStandardGlobalEnvironment(func(cfg environment.Configuration) environment.Configuration {
		cfg.Config = restConfig
		return cfg
	})

	flag.Parse()
	RunMainTest(m)
}

func merge(slices ...[]pkgupgrade.Operation) []pkgupgrade.Operation {
	l := 0
	for _, slice := range slices {
		l += len(slice)
	}
	result := make([]pkgupgrade.Operation, 0, l)
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}
