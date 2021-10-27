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

package state

import (
	"errors"
	"testing"
)

func TestStatus(t *testing.T) {
	testCases := []struct {
		name   string
		status *Status
		code   Code
		err    error
	}{
		{
			name:   "success",
			status: NewStatus(Success),
		},
		{
			name:   "error",
			status: NewStatus(Error),
			code:   Error,
		},
		{
			name:   "error as status",
			status: AsStatus(errors.New("invalid arguments")),
			code:   Error,
		},
		{
			name:   "unschedulable",
			status: NewStatus(Unschedulable, "invalid arguments"),
			code:   Unschedulable,
			err:    NewStatus(Unschedulable, "invalid arguments").AsError(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.status.IsSuccess() && tc.status.Code() != tc.code && tc.status.AsError() != tc.err {
				t.Errorf("unexpected code, got %v, want %v", tc.status.code, tc.code)
			} else if tc.status.IsUnschedulable() && tc.status.Code() != tc.code && tc.status.AsError() != tc.err {
				t.Errorf("unexpected code/msg, got %v, want %v, got %v, want %v", tc.status.code, tc.code, tc.status.AsError().Error(), tc.err.Error())
			} else if tc.status.IsError() && tc.status.Code() != tc.code && tc.status.AsError() != tc.err {
				t.Errorf("unexpected code/msg, got %v, want %v, got %v, want %v", tc.status.code, tc.code, tc.status.AsError().Error(), tc.err.Error())
			}
		})
	}
}

func TestStatusMerge(t *testing.T) {
	ps := PluginToStatus{"A": NewStatus(Success), "B": NewStatus(Success)}
	if !ps.Merge().IsSuccess() {
		t.Errorf("unexpected status from merge")
	}

	ps = PluginToStatus{"A": NewStatus(Success), "B": NewStatus(Error)}
	if !ps.Merge().IsError() {
		t.Errorf("unexpected status from merge")
	}

	ps = PluginToStatus{"A": NewStatus(Unschedulable), "B": NewStatus(Error)}
	if !ps.Merge().IsError() {
		t.Errorf("unexpected status from merge")
	}

	ps = PluginToStatus{"A": NewStatus(Unschedulable), "B": NewStatus(Success)}
	if !ps.Merge().IsUnschedulable() {
		t.Errorf("unexpected status from merge")
	}

}
