/*
Copyright 2018 The Knative Authors
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
	"context"
	"errors"

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/mattbaird/jsonpatch"
)

var (
	errInvalidFeedInput                = errors.New("failed to convert input into Feed")
	errInvalidFeedEventTypeMissing     = errors.New("the Feed must reference a EventType or ClusterEventType")
	errInvalidFeedEventTypeExclusivity = errors.New("the Feed must reference either a EventType or ClusterEventType, not both")
)

// Test the type for interface compliance
var _ GenericCRD = &v1alpha1.Feed{}

// ValidateFeed is the event type for a Feed
func ValidateFeed(ctx context.Context) ResourceCallback {
	return func(patches *[]jsonpatch.JsonPatchOperation, old GenericCRD, new GenericCRD) error {
		oldSubscription, newSubscription, err := unmarshalFeeds(ctx, old, new, "ValidateFeed")
		if err != nil {
			return err
		}

		return validateFeed(oldSubscription, newSubscription)
	}
}

func validateFeed(old, new *v1alpha1.Feed) error {
	refsEventType := len(new.Spec.Trigger.EventType) != 0
	refsClusterEventType := len(new.Spec.Trigger.ClusterEventType) != 0
	if !refsEventType && !refsClusterEventType {
		return errInvalidFeedEventTypeMissing
	} else if refsEventType && refsClusterEventType {
		return errInvalidFeedEventTypeExclusivity
	}
	// TODO(nicholss): write this.
	return nil
}

func unmarshalFeeds(
	ctx context.Context, old, new GenericCRD, fnName string) (*v1alpha1.Feed, *v1alpha1.Feed, error) {
	var oldFeed *v1alpha1.Feed
	if old != nil {
		var ok bool
		oldFeed, ok = old.(*v1alpha1.Feed)
		if !ok {
			return nil, nil, errInvalidSubscriptionInput
		}
	}
	glog.Infof("%s: OLD Feed is\n%+v", fnName, oldFeed)

	newFeed, ok := new.(*v1alpha1.Feed)
	if !ok {
		return nil, nil, errInvalidFeedInput
	}
	glog.Infof("%s: NEW Feed is\n%+v", fnName, newFeed)

	return oldFeed, newFeed, nil
}
