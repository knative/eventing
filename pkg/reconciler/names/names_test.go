/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package names

import (
	"testing"

	"knative.dev/pkg/network"
)

func TestNames(t *testing.T) {
	testCases := []struct {
		Name string
		F    func() string
		Want string
	}{{
		Name: "ServiceHostName",
		F: func() string {
			return ServiceHostName("foo", "namespace")
		},
		Want: "foo.namespace.svc." + network.GetClusterDomainName(),
	}}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if got := tc.F(); got != tc.Want {
				t.Errorf("want %v, got %v", tc.Want, got)
			}
		})
	}
}
