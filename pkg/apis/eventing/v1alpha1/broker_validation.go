/*
Copyright 2019 The Knative Authors

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
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (b *Broker) Validate() *apis.FieldError {
	return b.Spec.Validate().ViaField("spec")
}

func (bs *BrokerSpec) Validate() *apis.FieldError {
	var errs *apis.FieldError
	if isSelectorNotPresentOrEmpty(bs.Selector) {
		fe := apis.ErrMissingField("selector")
		fe.Details = "the Broker must have a selector"
		errs = errs.Also(fe)
	}

	if bs.ChannelTemplate == nil {
		fe := apis.ErrMissingField("channelTemplate")
		fe.Details = "the Broker must have a channelTemplate"
		errs = errs.Also(fe)
	} else if !selectorMatchesTemplateLabels(bs.Selector, bs.ChannelTemplate) {
		fe := apis.ErrInvalidValue(fmt.Sprint(bs.ChannelTemplate.metadata.Labels), "channelTemplate.metadata.labels")
		errs = errs.Also(fe)
	}

	if !channelsInSubscribableResources(bs.SubscribableResources) {
		fe := apis.ErrInvalidValue(fmt.Sprint(bs.SubscribableResources), "subscribableResources")
		errs = errs.Also(fe)
	}

	return errs
}

func isSelectorNotPresentOrEmpty(s *v1.LabelSelector) bool {
	return s == nil || equality.Semantic.DeepEqual(s, &v1.LabelSelector{})
}

func selectorMatchesTemplateLabels(s *v1.LabelSelector, ct *ChannelTemplateSpec) bool {
	// TODO Improve this so it supports something other than direct label equality.
	return equality.Semantic.DeepEqual(s.MatchLabels, ct.metadata.Labels)
}

func channelsInSubscribableResources(sr []v1.GroupVersionKind) bool {
	for _, r := range sr {
		if r.Group == "eventing.knative.dev" && r.Version == "v1alpha1" && r.Kind == "Channel" {
			return true
		}
	}
	return false
}

func (b *Broker) CheckImmutableFields(og apis.Immutable) *apis.FieldError {
	original, ok := og.(*Broker)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a Broker"}
	}
	if original == nil {
		return nil
	}

	if diff := cmp.Diff(original.Spec.Selector, b.Spec.Selector); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec", "selector"},
			Details: diff,
		}
	}
	return nil
}
