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
	errInvalidChannelInput              = errors.New("failed to convert input into Channel")
	errInvalidChannelBusMissing         = errors.New("the Channel must reference a Bus or ClusterBus")
	errInvalidChannelBusExclusivity     = errors.New("the Channel must reference either a Bus or ClusterBus, not both")
	errInvalidChannelBusMutation        = errors.New("the Channel's Bus may not change")
	errInvalidChannelClusterBusMutation = errors.New("the Channel's ClusterBus may not change")
)

// ValidateChannel is Channel resource specific validation and mutation handler
func ValidateChannel(ctx context.Context) ResourceCallback {
	return func(patches *[]jsonpatch.JsonPatchOperation, old GenericCRD, new GenericCRD) error {
		oldChannel, newChannel, err := unmarshalChannels(ctx, old, new, "ValidateChannel")
		if err != nil {
			return err
		}

		return validateChannel(oldChannel, newChannel)
	}
}

func validateChannel(old, new *v1alpha1.Channel) error {
	refsBus := len(new.Spec.Bus) != 0
	refsClusterBus := len(new.Spec.ClusterBus) != 0
	if !refsBus && !refsClusterBus {
		return errInvalidChannelBusMissing
	} else if refsBus && refsClusterBus {
		return errInvalidChannelBusExclusivity
	}
	if old != nil {
		if old.Spec.Bus != new.Spec.Bus {
			return errInvalidChannelBusMutation
		}
		if old.Spec.ClusterBus != new.Spec.ClusterBus {
			return errInvalidChannelClusterBusMutation
		}
	}
	return nil
}

func unmarshalChannels(
	ctx context.Context, old, new GenericCRD, fnName string) (*v1alpha1.Channel, *v1alpha1.Channel, error) {
	var oldChannel *v1alpha1.Channel
	if old != nil {
		var ok bool
		oldChannel, ok = old.(*v1alpha1.Channel)
		if !ok {
			return nil, nil, errInvalidChannelInput
		}
	}
	glog.Infof("%s: OLD Channel is\n%+v", fnName, oldChannel)

	newChannel, ok := new.(*v1alpha1.Channel)
	if !ok {
		return nil, nil, errInvalidChannelInput
	}
	glog.Infof("%s: NEW Channel is\n%+v", fnName, newChannel)

	return oldChannel, newChannel, nil
}
