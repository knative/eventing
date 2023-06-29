/*
Copyright 2023 The Knative Authors

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

package features

import (
	"fmt"

	"knative.dev/reconciler-test/pkg/eventshub"

	"knative.dev/eventing/pkg/apis"
)

func HasKnNamespaceHeader(ns string) eventshub.EventInfoMatcher {
	return func(info eventshub.EventInfo) error {
		values, ok := info.HTTPHeaders[apis.KnNamespaceHeader]
		if !ok {
			return fmt.Errorf("%s header not found", apis.KnNamespaceHeader)
		}
		for _, v := range values {
			if v == ns {
				return nil
			}
		}
		return fmt.Errorf("wanted %s header to have value %s, got %+v", apis.KnNamespaceHeader, ns, values)
	}
}
