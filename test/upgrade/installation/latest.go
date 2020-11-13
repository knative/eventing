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

package installation

import (
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

func LatestStable() pkgupgrade.Operation {
	return pkgupgrade.NewOperation("EventingLatestRelease", func(c pkgupgrade.Context) {
		shellfunc := "install_latest_release"
		c.Log.Info("Running shell function: ", shellfunc)
		err := callShellFunction(shellfunc)
		if err != nil {
			c.T.Error(err)
			return
		}
	})
}
