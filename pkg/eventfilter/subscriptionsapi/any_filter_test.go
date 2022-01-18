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

func TestAnyFilter_Flat(t *testing.T) {
	tests := map[string]struct {
		filter eventfilter.Filter
		want   eventfilter.FilterResult
	}{
		"Empty": {
			filter: NewAnyFilter(),
			want:   eventfilter.NoFilter,
		},
		"One Pass": {
			filter: NewAnyFilter(&passFilter{}),
			want:   eventfilter.PassFilter,
		},
		"One Fail": {
			filter: NewAnyFilter(&failFilter{}),
			want:   eventfilter.FailFilter,
		},
		"Pass and Fail": {
			filter: NewAnyFilter(&passFilter{}, &failFilter{}),
			want:   eventfilter.PassFilter,
		},
		"Pass and Pass": {
			filter: NewAnyFilter(&passFilter{}, &passFilter{}),
			want:   eventfilter.PassFilter,
		},
		"Fail and Fail": {
			filter: NewAnyFilter(&failFilter{}, &failFilter{}),
			want:   eventfilter.FailFilter,
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

func TestAnyFilter_WithNestedAny(t *testing.T) {
	tests := map[string]struct {
		filter eventfilter.Filter
		want   eventfilter.FilterResult
	}{
		"Any(Any(Pass))": {
			filter: NewAnyFilter(
				NewAnyFilter(&passFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"Any(Any(Fail))": {
			filter: NewAnyFilter(
				NewAnyFilter(&failFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"Any(Pass,Any(Fail))": {
			filter: NewAnyFilter(&passFilter{},
				NewAnyFilter(&failFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"Any(Pass,Any(Pass))": {
			filter: NewAnyFilter(&passFilter{},
				NewAnyFilter(&passFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"Any(Fail,Any(Fail))": {
			filter: NewAnyFilter(&failFilter{},
				NewAnyFilter(&failFilter{}),
			),
			want: eventfilter.FailFilter,
		},
		"Any(Fail,Any(Pass))": {
			filter: NewAnyFilter(&failFilter{},
				NewAnyFilter(&passFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"Any(Pass,Any(Pass,Fail))": {
			filter: NewAnyFilter(&passFilter{},
				NewAnyFilter(&passFilter{}, &failFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"Any(Pass,Any(Pass,Pass))": {
			filter: NewAnyFilter(&passFilter{},
				NewAnyFilter(&passFilter{}, &passFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"Any(Pass,Any(Fail,Fail))": {
			filter: NewAnyFilter(&passFilter{},
				NewAnyFilter(&failFilter{}, &failFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"Any(Fail,Any(Pass,Fail))": {
			filter: NewAnyFilter(&failFilter{},
				NewAnyFilter(&passFilter{}, &failFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"Any(Fail,Any(Pass,Pass))": {
			filter: NewAnyFilter(&failFilter{},
				NewAnyFilter(&passFilter{}, &passFilter{}),
			),
			want: eventfilter.PassFilter,
		},
		"Any(Fail,Any(Fail,Fail))": {
			filter: NewAnyFilter(&failFilter{},
				NewAnyFilter(&failFilter{}, &failFilter{}),
			),
			want: eventfilter.FailFilter,
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
