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

func TestNewFeedNSEventType(t *testing.T) {
	c := createFeed(testFeedName, testEventTypeName, "")
	if err := ValidateFeed(testCtx)(nil, nil, &c); err != nil {
		t.Errorf("Expected success, but failed with: %s", err)
	}
}

func TestNewFeedClusterEventType(t *testing.T) {
	c := createFeed(testFeedName, "", testClusterEventTypeName)
	if err := ValidateFeed(testCtx)(nil, nil, &c); err != nil {
		t.Errorf("Expected success, but failed with: %s", err)
	}
}

func TestNewEmptyFeed(t *testing.T) {
	c := createFeed(testFeedName, "", "")
	err := ValidateFeed(testCtx)(nil, nil, &c)
	if err == nil {
		t.Errorf("Expected failure, but succeeded with: %+v", c)
	}
	if e, a := errInvalidFeedEventTypeMissing, err; e != a {
		t.Errorf("Expected %s got %s", e, a)
	}
}

func TestNewExclusiveFeed(t *testing.T) {
	c := createFeed(testFeedName, testEventTypeName, testClusterEventTypeName)
	err := ValidateFeed(testCtx)(nil, nil, &c)
	if err == nil {
		t.Errorf("Expected failure, but succeeded with: %+v", c)
	}
	if e, a := errInvalidFeedEventTypeExclusivity, err; e != a {
		t.Errorf("Expected %s got %s", e, a)
	}
}
