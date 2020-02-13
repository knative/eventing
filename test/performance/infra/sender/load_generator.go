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

package sender

import (
	"knative.dev/eventing/test/performance/infra/common"
)

type LoadGenerator interface {
	// This method blocks till the warmup is complete
	Warmup(pace common.PaceSpec, msgSize uint, fixedBody bool)

	// This method blocks till the pace is complete
	RunPace(i int, pace common.PaceSpec, msgSize uint, fixedBody bool)
	SendGCEvent()
	SendEndEvent()
}

type LoadGeneratorFactory func(eventSource string, sentCh chan common.EventTimestamp,
	acceptedCh chan common.EventTimestamp) (LoadGenerator, error)
