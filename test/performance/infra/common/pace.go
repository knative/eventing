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

package common

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const defaultPaceDuration = 10 * time.Second

type PaceSpec struct {
	Rps      int
	Duration time.Duration
}

// We need those estimates to allocate memory before benchmark starts
func CalculateMemoryConstraintsForPaceSpecs(paceSpecs []PaceSpec) (estimatedNumberOfMessagesInsideAChannel uint64, estimatedNumberOfTotalMessages uint64) {
	for _, pacer := range paceSpecs {
		totalMessages := uint64(pacer.Rps * int(pacer.Duration.Seconds()))
		// Add a bit more, just to be sure that we don't under allocate
		totalMessages = totalMessages + uint64(float64(totalMessages)*0.1)
		// Aggressively set the queue length so enqueue operation won't be blocked
		// as the total number of messages grows.
		queueLength := uint64(pacer.Rps * 5)
		estimatedNumberOfTotalMessages += totalMessages
		if queueLength > estimatedNumberOfMessagesInsideAChannel {
			estimatedNumberOfMessagesInsideAChannel = queueLength
		}
	}
	return
}

func ParsePaceSpec(pace string) ([]PaceSpec, error) {
	paceSpecArray := strings.Split(pace, ",")
	pacerSpecs := make([]PaceSpec, 0)

	for _, p := range paceSpecArray {
		ps := strings.Split(p, ":")
		rps, err := strconv.Atoi(ps[0])
		if err != nil {
			return nil, fmt.Errorf("invalid format %q: %v", ps, err)
		}
		duration := defaultPaceDuration

		if len(ps) == 2 {
			durationSec, err := strconv.Atoi(ps[1])
			if err != nil {
				return nil, fmt.Errorf("invalid format %q: %v", ps, err)
			}
			duration = time.Second * time.Duration(durationSec)
		}

		pacerSpecs = append(pacerSpecs, PaceSpec{rps, duration})
	}

	return pacerSpecs, nil
}
