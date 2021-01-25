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
	"testing"

	"go.uber.org/zap"
	"knative.dev/eventing/test/upgrade/installation"
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

func TestEventingUpgrades(t *testing.T) {
	suite := pkgupgrade.Suite{
		Tests: pkgupgrade.Tests{
			PreUpgrade: []pkgupgrade.Operation{
				PreUpgradeTest(),
			},
			PostUpgrade: []pkgupgrade.Operation{
				PostUpgradeTest(),
			},
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
	c := newUpgradeConfig(t)
	suite.Execute(c)
}

func newUpgradeConfig(t *testing.T) pkgupgrade.Configuration {
	log, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err)
	}
	return pkgupgrade.Configuration{T: t, Log: log}
}

func TestMain(m *testing.M) {
	RunMainTest(m)
}
