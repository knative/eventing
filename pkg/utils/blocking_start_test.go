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

package utils

import (
	"errors"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"testing"
	"time"
)

var (
	errorFromImmediateError = errors.New("error from immediate error")
)

func TestBlockingStart(t *testing.T) {
	testCases := map[string]struct {
		f        manager.RunnableFunc
		expected error
	}{
		"error exits immediately": {
			f:        immediateError,
			expected: errorFromImmediateError,
		},
		"nil blocks until stopCh closes": {
			f:        blockUntilStopChCloses,
			expected: nil,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			runnable := &runnableWrapper{f: tc.f}
			sb := NewBlockingStart(zap.NewNop(), runnable)

			stopCh := make(chan struct{})
			stopChClosed := false
			defer func() {
				if !stopChClosed {
					close(stopCh)
				}
			}()

			returned := make(chan error)

			go func() {
				returned <- sb.Start(stopCh)
			}()

			for i := 0; i < 2; i++ {
				timer := time.NewTimer(50 * time.Millisecond)
				select {
				case <-timer.C:
					if tc.expected != nil {
						t.Fatalf("Start() should have returned an error before the time ran out")
					}
					if stopChClosed {
						t.Fatalf("The stop channel has already been closed, we should not have reached a timeout again")
					}
					// Close the stop channel then let the loop run again. While waiting for the next timer,
					// blockingStart should finish.
					stopChClosed = true
					close(stopCh)
				case actual := <-returned:
					if tc.expected != actual {
						t.Errorf("Unexpected response. Expected %v. Actual %v.", tc.expected, actual)
					}
					return
				}
			}
			t.Fatalf("Exited the for loop without returning.")
		})
	}
}

type runnableWrapper struct {
	f func(<-chan struct{}) error
}

func (r *runnableWrapper) Start(stopCh <-chan struct{}) error {
	return r.f(stopCh)
}

func immediateError(_ <-chan struct{}) error {
	return errorFromImmediateError
}

func blockUntilStopChCloses(stopCh <-chan struct{}) error {
	<-stopCh
	return nil
}
