/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package event

import "knative.dev/eventing/test/upgrade/prober/wathola/config"

const (
	// StepType is a string type representation of step event
	StepType = "com.github.cardil.wathola.step"
	// FinishedType os a string type representation of finished event
	FinishedType = "com.github.cardil.wathola.finished"
)

// Step is a event call at each step of verification
type Step struct {
	Number int
}

// Finished is step call after verification finishes
type Finished struct {
	Count int
}

// Type returns a type of a event
func (s Step) Type() string {
	return StepType
}

// Type returns a type of a event
func (f Finished) Type() string {
	return FinishedType
}

// State defines a state of event store
type State int

const (
	// Active == 1 (iota has been reset)
	Active State = 1 << iota
	// Success == 2
	Success State = 1 << iota
	// Failed == 4
	Failed State = 1 << iota
)

var log = config.Log
