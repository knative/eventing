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

package kncloudevents

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

// Test The RetryConfigFromDeliverySpec() Functionality
func TestRetryConfigFromDeliverySpec(t *testing.T) {

	// Define The TestCase Structure
	type TestCase struct {
		name                     string
		retry                    int32
		backoffPolicy            duckv1.BackoffPolicyType
		backoffDelay             string
		expectedBackoffDurations []time.Duration
		wantErr                  bool
	}

	// Create The TestCases
	testcases := []TestCase{
		{
			name:          "Successful Linear Backoff 2500ms",
			retry:         int32(5),
			backoffPolicy: duckv1.BackoffPolicyLinear,
			backoffDelay:  "PT2.5S",
			expectedBackoffDurations: []time.Duration{
				2500 * time.Millisecond,
				2500 * time.Millisecond,
				2500 * time.Millisecond,
				2500 * time.Millisecond,
				2500 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name:          "Successful Exponential Backoff 1500ms",
			retry:         int32(5),
			backoffPolicy: duckv1.BackoffPolicyExponential,
			backoffDelay:  "PT1.5S",
			expectedBackoffDurations: []time.Duration{
				3 * time.Second,
				6 * time.Second,
				12 * time.Second,
				24 * time.Second,
				48 * time.Second,
			},
			wantErr: false,
		},
		{
			name:          "Successful Exponential Backoff 500ms",
			retry:         int32(5),
			backoffPolicy: duckv1.BackoffPolicyExponential,
			backoffDelay:  "PT0.5S",
			expectedBackoffDurations: []time.Duration{
				1 * time.Second,
				2 * time.Second,
				4 * time.Second,
				8 * time.Second,
				16 * time.Second,
			},
			wantErr: false,
		},
		{
			name:          "Invalid Backoff Delay",
			retry:         int32(5),
			backoffPolicy: duckv1.BackoffPolicyLinear,
			backoffDelay:  "FOO",
			wantErr:       true,
		},
	}

	// Loop Over The TestCases
	for _, testcase := range testcases {

		// Execute The TestCase
		t.Run(testcase.name, func(t *testing.T) {

			// Create The DeliverySpec To Test
			deliverySpec := duckv1.DeliverySpec{
				DeadLetterSink: nil,
				Retry:          &testcase.retry,
				BackoffPolicy:  &testcase.backoffPolicy,
				BackoffDelay:   &testcase.backoffDelay,
			}

			// Create The RetryConfig From The DeliverySpec
			retryConfig, err := RetryConfigFromDeliverySpec(deliverySpec)
			assert.Equal(t, testcase.wantErr, err != nil)

			// If Successful Then Validate The RetryConfig (Max & Backoff Calculations)
			if err == nil {
				assert.Equal(t, int(testcase.retry), retryConfig.RetryMax)
				for i := 1; i < int(testcase.retry); i++ {
					expectedBackoffDuration := testcase.expectedBackoffDurations[i-1]
					actualBackoffDuration := retryConfig.Backoff(i, nil)
					assert.Equal(t, expectedBackoffDuration, actualBackoffDuration)
				}
			}
		})
	}
}
