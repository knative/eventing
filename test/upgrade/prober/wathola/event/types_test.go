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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStepType(t *testing.T) {
	s1 := Step{Number: 1}

	assert.Equal(t, StepType, s1.Type())
}

func TestFinishedType(t *testing.T) {
	f1 := Finished{EventsSent: 441}

	assert.Equal(t, FinishedType, f1.Type())
}
