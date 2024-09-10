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
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/test/migrate"
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

func CRDPostUpgradeTest() pkgupgrade.Operation {
	return pkgupgrade.NewOperation("PostUpgradeCRDTest", func(c pkgupgrade.Context) {
		client := testlib.Setup(c.T, true)
		defer testlib.TearDown(client)
		migrate.ExpectSingleStoredVersion(c.T, client.Apiextensions.CustomResourceDefinitions(), "knative.dev")
	})
}
