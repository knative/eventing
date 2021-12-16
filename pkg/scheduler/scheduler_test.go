/*
Copyright 2020 The Knative Authors

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

package scheduler

import (
	"testing"

	"github.com/stretchr/testify/require"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
)

func TestSchedulerFuncSchedule(t *testing.T) {

	called := 0

	var s Scheduler = SchedulerFunc(func(vpod VPod) ([]duckv1alpha1.Placement, error) {
		called++
		return nil, nil
	})

	_, err := s.Schedule(nil)
	require.Nil(t, err)
	require.Equal(t, 1, called)
}
