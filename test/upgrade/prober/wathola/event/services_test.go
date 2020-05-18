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

import (
	"github.com/stretchr/testify/assert"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"

	"os"
	"testing"
	"time"
)

func TestProperEventsPropagation(t *testing.T) {
	// given
	errors := NewErrorStore()
	stepsStore := NewStepsStore(errors)
	finishedStore := NewFinishedStore(stepsStore, errors)

	// when
	stepsStore.RegisterStep(&Step{Number: 1})
	stepsStore.RegisterStep(&Step{Number: 3})
	stepsStore.RegisterStep(&Step{Number: 2})
	finishedStore.RegisterFinished(&Finished{Count: 3})

	// then
	assert.Empty(t, errors.thrown)
}

func TestMissingAndDoubleEvent(t *testing.T) {
	// given
	errors := NewErrorStore()
	stepsStore := NewStepsStore(errors)
	finishedStore := NewFinishedStore(stepsStore, errors)

	// when
	stepsStore.RegisterStep(&Step{Number: 1})
	stepsStore.RegisterStep(&Step{Number: 2})
	stepsStore.RegisterStep(&Step{Number: 2})
	finishedStore.RegisterFinished(&Finished{Count: 3})

	// then
	assert.NotEmpty(t, errors.thrown)
}

func TestDoubleFinished(t *testing.T) {
	// given
	errors := NewErrorStore()
	stepsStore := NewStepsStore(errors)
	finishedStore := NewFinishedStore(stepsStore, errors)

	// when
	stepsStore.RegisterStep(&Step{Number: 1})
	stepsStore.RegisterStep(&Step{Number: 2})
	finishedStore.RegisterFinished(&Finished{Count: 2})
	finishedStore.RegisterFinished(&Finished{Count: 2})

	// then
	assert.NotEmpty(t, errors.thrown)
}

func TestMain(m *testing.M) {
	config.Instance.Receiver.Teardown.Duration = 20 * time.Millisecond
	exitcode := m.Run()
	os.Exit(exitcode)
}
