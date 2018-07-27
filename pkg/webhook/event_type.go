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
	errInvalidEventTypeInput = errors.New("failed to convert input into EventType")
)

// Test the type for interface compliance
var _ GenericCRD = &v1alpha1.EventType{}

// ValidateEventType is the event type for a Feed
func ValidateEventType(ctx context.Context) ResourceCallback {
	return func(patches *[]jsonpatch.JsonPatchOperation, old GenericCRD, new GenericCRD) error {
		oldSubscription, newSubscription, err := unmarshalEventTypes(ctx, old, new, "ValidateEventType")
		if err != nil {
			return err
		}

		return validateEventType(oldSubscription, newSubscription)
	}
}

func validateEventType(old, new *v1alpha1.EventType) error {
	// TODO(nicholss): write this.
	return nil
}

func unmarshalEventTypes(
	ctx context.Context, old, new GenericCRD, fnName string) (*v1alpha1.EventType, *v1alpha1.EventType, error) {
	var oldEventType *v1alpha1.EventType
	if old != nil {
		var ok bool
		oldEventType, ok = old.(*v1alpha1.EventType)
		if !ok {
			return nil, nil, errInvalidSubscriptionInput
		}
	}
	glog.Infof("%s: OLD EventType is\n%+v", fnName, oldEventType)

	newEventType, ok := new.(*v1alpha1.EventType)
	if !ok {
		return nil, nil, errInvalidEventTypeInput
	}
	glog.Infof("%s: NEW EventType is\n%+v", fnName, newEventType)

	return oldEventType, newEventType, nil
}
