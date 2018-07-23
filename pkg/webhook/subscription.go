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
	"github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/mattbaird/jsonpatch"
)

var (
	errInvalidSubscriptionInput           = errors.New("failed to convert input into Subscription")
	errInvalidSubscriptionChannelMissing  = errors.New("the Subscription must reference a Channel")
	errInvalidSubscriptionChannelMutation = errors.New("the Subscription's Channel may not change")
)

// ValidateSubscription is Subscription resource specific validation and mutation handler
func ValidateSubscription(ctx context.Context) ResourceCallback {
	return func(patches *[]jsonpatch.JsonPatchOperation, old GenericCRD, new GenericCRD) error {
		oldSubscription, newSubscription, err := unmarshalSubscriptions(ctx, old, new, "ValidateSubscription")
		if err != nil {
			return err
		}

		return validateSubscription(oldSubscription, newSubscription)
	}
}

func validateSubscription(old, new *v1alpha1.Subscription) error {
	if len(new.Spec.Channel) == 0 {
		return errInvalidSubscriptionChannelMissing
	}
	if old != nil && old.Spec.Channel != new.Spec.Channel {
		return errInvalidSubscriptionChannelMutation
	}
	return nil
}

func unmarshalSubscriptions(
	ctx context.Context, old, new GenericCRD, fnName string) (*v1alpha1.Subscription, *v1alpha1.Subscription, error) {
	var oldSubscription *v1alpha1.Subscription
	if old != nil {
		var ok bool
		oldSubscription, ok = old.(*v1alpha1.Subscription)
		if !ok {
			return nil, nil, errInvalidSubscriptionInput
		}
	}
	glog.Infof("%s: OLD Subscription is\n%+v", fnName, oldSubscription)

	newSubscription, ok := new.(*v1alpha1.Subscription)
	if !ok {
		return nil, nil, errInvalidSubscriptionInput
	}
	glog.Infof("%s: NEW Subscription is\n%+v", fnName, newSubscription)

	return oldSubscription, newSubscription, nil
}
