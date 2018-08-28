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

package v1alpha1

import (
	"github.com/knative/pkg/apis"

	"k8s.io/apimachinery/pkg/util/validation"
	"strings"
)

func (f *Feed) Validate() *apis.FieldError {
	return f.Spec.Validate().ViaField("spec")
}

func (fs *FeedSpec) Validate() *apis.FieldError {
	if err := fs.Trigger.Validate(); err != nil {
		return err.ViaField("trigger")
	}
	if err := fs.Action.Validate(); err != nil {
		return err.ViaField("action")
	}
	return nil
}

func (et *EventTrigger) Validate() *apis.FieldError {
	switch {
	case len(et.EventType) != 0 && len(et.ClusterEventType) != 0:
		return apis.ErrMultipleOneOf("eventType", "clusterEventType")
	case len(et.EventType) != 0:
		if errs := validation.IsQualifiedName(et.EventType); len(errs) > 0 {
			return apis.ErrInvalidKeyName(et.EventType, "eventType", errs...)
		}
	case len(et.ClusterEventType) != 0:
		if errs := validation.IsQualifiedName(et.ClusterEventType); len(errs) > 0 {
			return apis.ErrInvalidKeyName(et.ClusterEventType, "clusterEventType", errs...)
		}
	default:
		return apis.ErrMissingOneOf("eventType", "clusterEventType")
	}

	if et.Resource == "" {
		return apis.ErrMissingField("resource")
	}

	return nil
}

func (fa *FeedAction) Validate() *apis.FieldError {
	if len(fa.DNSName) != 0 {
		if errs := validation.IsDNS1123Subdomain(fa.DNSName); len(errs) > 0 {
			err := apis.ErrInvalidValue(fa.DNSName, "dnsName")
			err.Details = strings.Join(errs, ", ")
			return err
		}
	}
	return nil
}
