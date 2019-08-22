/*
Copyright 2019 The Knative Authors

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

package main

import (
	"fmt"
	"strconv"
	"testing"
)

func TestCalculateSamplePercentile(t *testing.T) {
	values := []int64{1, 3, 4, 2, 3, 4, 6, 7, 3, 5, 62, 4, 6, 3, 8, 54, 52, 45}
	tests := []struct {
		perc float64
		res  float64
	}{{
		perc: 0.2,
		res:  3.000000,
	}, {
		perc: 0.5,
		res:  4.500000,
	}, {
		perc: 0.7,
		res:  7.299999999999999,
	}, {
		perc: 0.8,
		res:  46.400000000000006,
	}, {
		perc: 0.9,
		res:  54.80000000000001,
	}}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s-%f", t.Name(), test.perc), func(t *testing.T) {
			got := calculateSamplePercentile(values, test.perc)
			if test.res != got {
				t.Errorf(
					"unexpected res for %f sample percentile of %v, wanted %s, got %s",
					test.perc,
					values,
					strconv.FormatFloat(test.res, 'f', -1, 64),
					strconv.FormatFloat(got, 'f', -1, 64),
				)
			}
		})
	}
}
