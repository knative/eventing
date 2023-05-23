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
	"testing"

	"knative.dev/eventing/test"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/upgrade/installation"
	"knative.dev/pkg/system"
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

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

	suite := pkgupgrade.Suite{
		Tests: pkgupgrade.Tests{
			PreUpgrade: []pkgupgrade.Operation{
				PreUpgradeTest(),
			},
			PostUpgrade: PostUpgradeTests(),
			PostDowngrade: []pkgupgrade.Operation{
				PostDowngradeTest(),
			},
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
	flag.Parse()
	RunMainTest(m)
}
