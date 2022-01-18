/*
Copyright 2022 The Knative Authors

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

package subscriptionsapi

import (
	"context"
	"testing"

	"knative.dev/eventing/pkg/eventfilter"
)

func TestNotFilter(t *testing.T) {
	tests := map[string]struct {
		filter eventfilter.Filter
		want   eventfilter.FilterResult
	}{
		"Empty": {
			filter: &notFilter{},
			want:   eventfilter.NoFilter,
		},
		"Not Pass": {
			filter: NewNotFilter(&passFilter{}),
			want:   eventfilter.FailFilter,
		},
		"Not Fail": {
			filter: NewNotFilter(&failFilter{}),
			want:   eventfilter.PassFilter,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := makeEvent()
			if got := tc.filter.Filter(context.TODO(), *e); got != tc.want {
				t.Errorf("Filter() = %s, want %s", got, tc.want)
			}
		})
	}
}
