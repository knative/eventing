/*
 * Copyright 2019 The Knative Authors
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

package broker

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBuckets125(t *testing.T) {
	testCases := []struct {
		low  float64
		high float64
		want []float64
	}{{
		low:  1,
		high: 100,
		want: []float64{1, 2, 5, 10, 20, 50, 100},
	}, {
		low:  0.1,
		high: 10,
		want: []float64{0.1, 0.2, 0.5, 1, 2, 5, 10},
	}}

	for i, tc := range testCases {
		t.Run(string(i), func(t *testing.T) {
			got := Buckets125(tc.low, tc.high)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected buckets for %f-%f (-want, +got): %s", tc.low, tc.high, diff)
			}
		})
	}
}
