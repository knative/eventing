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
	"slices"
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

	g := FeatureGroupWithUpgradeTests{
		// A feature that will run the same test post-upgrade and post-downgrade.
		NewFeatureSmoke(InMemoryChannelFeature(global)),
		// A feature that will be created pre-upgrade and verified/removed post-upgrade.
		NewFeatureOnlyUpgrade(InMemoryChannelFeature(global)),
		// A feature that will be created pre-upgrade, verified post-upgrade, verified and removed post-downgrade.
		NewFeatureUpgradeDowngrade(InMemoryChannelFeature(global)),
		// A feature that will be created post-upgrade, verified and removed post-downgrade.
		NewFeatureOnlyDowngrade(InMemoryChannelFeature(global)),
	}

	suite := pkgupgrade.Suite{
		Tests: pkgupgrade.Tests{
			PreUpgrade: slices.Concat(
				g.PreUpgradeTests(),
			),
			PostUpgrade: slices.Concat(
				[]pkgupgrade.Operation{
					CRDPostUpgradeTest(),
				},
				g.PostUpgradeTests(),
			),
			PostDowngrade: slices.Concat(
				g.PostDowngradeTests(),
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
