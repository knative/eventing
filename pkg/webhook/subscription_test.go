/*
Copyright 2018 Google LLC. All Rights Reserved.
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

package webhook

import (
	"testing"
)

func TestNewSubscription(t *testing.T) {
	s := createSubscription(testSubscriptionName, testChannelName)
	if err := ValidateSubscription(testCtx)(nil, nil, &s); err != nil {
		t.Errorf("Expected success, but failed with: %s", err)
	}
}

func TestNewEmptySubscription(t *testing.T) {
	s := createSubscription(testSubscriptionName, "")
	err := ValidateSubscription(testCtx)(nil, nil, &s)
	if err == nil {
		t.Errorf("Expected failure, but succeeded with: %+v", s)
	}
	if e, a := errInvalidSubscriptionChannelMissing, err; e != a {
		t.Errorf("Expected %s got %s", e, a)
	}
}

func TestSubscriptionMutation(t *testing.T) {
	s := createSubscription(testSubscriptionName, testChannelName)
	if err := ValidateSubscription(testCtx)(nil, &s, &s); err != nil {
		t.Errorf("Expected success, but failed with: %s", err)
	}
}

func TestSubscriptionChannelMutation(t *testing.T) {
	old := createSubscription(testSubscriptionName, "hello")
	new := createSubscription(testSubscriptionName, "goodbye")
	err := ValidateSubscription(testCtx)(nil, &old, &new)
	if err == nil {
		t.Errorf("Expected failure, but succeeded with: %+v %+v", old, new)
	}
	if e, a := errInvalidSubscriptionChannelMutation, err; e != a {
		t.Errorf("Expected %s got %s", e, a)
	}
}
