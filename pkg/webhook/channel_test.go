/*
Copyright 2018 The Knative Authors. All Rights Reserved.
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

// TODO(n3wscott): these tests will go away when we move to new type of webhooks.

func TestNewChannelNSBus(t *testing.T) {
	c := createChannel(testChannelName, testBusName, "")
	if err := ValidateChannel(testCtx)(nil, nil, &c); err != nil {
		t.Errorf("Expected success, but failed with: %s", err)
	}
}

func TestNewChannelClusterBus(t *testing.T) {
	c := createChannel(testChannelName, "", testClusterBusName)
	if err := ValidateChannel(testCtx)(nil, nil, &c); err != nil {
		t.Errorf("Expected success, but failed with: %s", err)
	}
}

func TestNewEmptyChannel(t *testing.T) {
	c := createChannel(testChannelName, "", "")
	err := ValidateChannel(testCtx)(nil, nil, &c)
	if err == nil {
		t.Errorf("Expected failure, but succeeded with: %+v", c)
	}
}

func TestNewExclusiveChannel(t *testing.T) {
	c := createChannel(testChannelName, testBusName, testClusterBusName)
	err := ValidateChannel(testCtx)(nil, nil, &c)
	if err == nil {
		t.Errorf("Expected failure, but succeeded with: %+v", c)
	}
}

func TestChannelNoopMutation(t *testing.T) {
	c := createChannel(testChannelName, testBusName, "")
	if err := ValidateChannel(testCtx)(nil, &c, &c); err != nil {
		t.Errorf("Expected success, but failed with: %s", err)
	}
}

func TestChannelNSBusMutation(t *testing.T) {
	old := createChannel(testChannelName, "stub", "")
	new := createChannel(testChannelName, "pubsub", "")
	err := ValidateChannel(testCtx)(nil, &old, &new)
	if err == nil {
		t.Errorf("Expected failure, but succeeded with: %+v %+v", old, new)
	}
}

func TestChannelClusterBusMutation(t *testing.T) {
	old := createChannel(testChannelName, "", "stub")
	new := createChannel(testChannelName, "", "pubsub")
	err := ValidateChannel(testCtx)(nil, &old, &new)
	if err == nil {
		t.Errorf("Expected failure, but succeeded with: %+v %+v", old, new)
	}
}
