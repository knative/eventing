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
	errInvalidClusterEventTypeInput = errors.New("failed to convert input into ClusterEventType")
)

// Test the type for interface compliance
var _ GenericCRD = &v1alpha1.ClusterEventType{}

// ValidateClusterEventType is the event type for a Feed
func ValidateClusterEventType(ctx context.Context) ResourceCallback {
	return func(patches *[]jsonpatch.JsonPatchOperation, old GenericCRD, new GenericCRD) error {
		oldSubscription, newSubscription, err := unmarshalClusterEventTypes(ctx, old, new, "ValidateClusterEventType")
		if err != nil {
			return err
		}

		return validateClusterEventType(oldSubscription, newSubscription)
	}
}

func validateClusterEventType(old, new *v1alpha1.ClusterEventType) error {
	// TODO(nicholss): write this.
	return nil
}

func unmarshalClusterEventTypes(
	ctx context.Context, old, new GenericCRD, fnName string) (*v1alpha1.ClusterEventType, *v1alpha1.ClusterEventType, error) {
	var oldClusterEventType *v1alpha1.ClusterEventType
	if old != nil {
		var ok bool
		oldClusterEventType, ok = old.(*v1alpha1.ClusterEventType)
		if !ok {
			return nil, nil, errInvalidSubscriptionInput
		}
	}
	glog.Infof("%s: OLD ClusterEventType is\n%+v", fnName, oldClusterEventType)

	newClusterEventType, ok := new.(*v1alpha1.ClusterEventType)
	if !ok {
		return nil, nil, errInvalidClusterEventTypeInput
	}
	glog.Infof("%s: NEW ClusterEventType is\n%+v", fnName, newClusterEventType)

	return oldClusterEventType, newClusterEventType, nil
}
