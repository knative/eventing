/*
Copyright 2020 The Knative Authors.

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

package v1alpha1

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/flows/v1beta1"
)

// ConvertTo implements apis.Convertible
// Converts obj from v1alpha1.Parallel into v1beta1.Parallel
func (source *Parallel) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.Parallel:
		sink.ObjectMeta = source.ObjectMeta

		sink.Spec.Branches = make([]v1beta1.ParallelBranch, len(source.Spec.Branches))
		for i, b := range source.Spec.Branches {
			sink.Spec.Branches[i] = v1beta1.ParallelBranch{
				Filter:     b.Filter,
				Subscriber: b.Subscriber,
				Reply:      b.Reply,
			}
		}

		sink.Spec.ChannelTemplate = source.Spec.ChannelTemplate
		sink.Spec.Reply = source.Spec.Reply

		sink.Status.Status = source.Status.Status
		sink.Status.AddressStatus = source.Status.AddressStatus

		sink.Status.IngressChannelStatus = v1beta1.ParallelChannelStatus{
			Channel:        source.Status.IngressChannelStatus.Channel,
			ReadyCondition: source.Status.IngressChannelStatus.ReadyCondition,
		}

		if source.Status.BranchStatuses != nil {
			sink.Status.BranchStatuses = make([]v1beta1.ParallelBranchStatus, len(source.Status.BranchStatuses))
			for i, b := range source.Status.BranchStatuses {
				sink.Status.BranchStatuses[i] = v1beta1.ParallelBranchStatus{
					FilterSubscriptionStatus: v1beta1.ParallelSubscriptionStatus{
						Subscription:   b.FilterSubscriptionStatus.Subscription,
						ReadyCondition: b.FilterSubscriptionStatus.ReadyCondition,
					},
					FilterChannelStatus: v1beta1.ParallelChannelStatus{
						Channel:        b.FilterChannelStatus.Channel,
						ReadyCondition: b.FilterChannelStatus.ReadyCondition,
					},
					SubscriptionStatus: v1beta1.ParallelSubscriptionStatus{
						Subscription:   b.SubscriptionStatus.Subscription,
						ReadyCondition: b.SubscriptionStatus.ReadyCondition,
					},
				}
			}
		}

		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)
	}
}

// ConvertFrom implements apis.Convertible
// Converts obj from v1beta1.Sequence into v1alpha1.Sequence
func (sink *Parallel) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.Parallel:
		sink.ObjectMeta = source.ObjectMeta

		sink.Spec.Branches = make([]ParallelBranch, len(source.Spec.Branches))
		for i, b := range source.Spec.Branches {
			sink.Spec.Branches[i] = ParallelBranch{
				Filter:     b.Filter,
				Subscriber: b.Subscriber,
				Reply:      b.Reply,
			}
		}

		sink.Spec.ChannelTemplate = source.Spec.ChannelTemplate
		sink.Spec.Reply = source.Spec.Reply

		sink.Status.Status = source.Status.Status
		sink.Status.AddressStatus = source.Status.AddressStatus

		sink.Status.IngressChannelStatus = ParallelChannelStatus{
			Channel:        source.Status.IngressChannelStatus.Channel,
			ReadyCondition: source.Status.IngressChannelStatus.ReadyCondition,
		}

		if source.Status.BranchStatuses != nil {
			sink.Status.BranchStatuses = make([]ParallelBranchStatus, len(source.Status.BranchStatuses))
			for i, b := range source.Status.BranchStatuses {
				sink.Status.BranchStatuses[i] = ParallelBranchStatus{
					FilterSubscriptionStatus: ParallelSubscriptionStatus{
						Subscription:   b.FilterSubscriptionStatus.Subscription,
						ReadyCondition: b.FilterSubscriptionStatus.ReadyCondition,
					},
					FilterChannelStatus: ParallelChannelStatus{
						Channel:        b.FilterChannelStatus.Channel,
						ReadyCondition: b.FilterChannelStatus.ReadyCondition,
					},
					SubscriptionStatus: ParallelSubscriptionStatus{
						Subscription:   b.SubscriptionStatus.Subscription,
						ReadyCondition: b.SubscriptionStatus.ReadyCondition,
					},
				}
			}
		}

		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", source)
	}
}
