/*
Copyright 2024 The Knative Authors

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

package subscriptionsapi

import (
	"go.uber.org/zap"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter"
)

// MaterializeSubscriptionsAPIFilter materializes a SubscriptionsAPIFilter into a runnable Filter.
func MaterializeSubscriptionsAPIFilter(logger *zap.Logger, filter v1.SubscriptionsAPIFilter) eventfilter.Filter {
	var materializedFilter eventfilter.Filter
	var err error
	switch {
	case len(filter.Exact) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = NewExactFilter(filter.Exact)
		if err != nil {
			logger.Debug("Invalid exact expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Prefix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = NewPrefixFilter(filter.Prefix)
		if err != nil {
			logger.Debug("Invalid prefix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Suffix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = NewSuffixFilter(filter.Suffix)
		if err != nil {
			logger.Debug("Invalid suffix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.All) > 0:
		materializedFilter = NewAllFilter(MaterializeFiltersList(logger, filter.All)...)
	case len(filter.Any) > 0:
		materializedFilter = NewAnyFilter(MaterializeFiltersList(logger, filter.Any)...)
	case filter.Not != nil:
		materializedFilter = NewNotFilter(MaterializeSubscriptionsAPIFilter(logger, *filter.Not))
	case filter.CESQL != "":
		if materializedFilter, err = NewCESQLFilter(filter.CESQL); err != nil {
			// This is weird, CESQL expression should be validated when Trigger's are created.
			logger.Debug("Found an Invalid CE SQL expression", zap.String("expression", filter.CESQL))
			return nil
		}
	}
	return materializedFilter
}

func CreateSubscriptionsAPIFilters(logger *zap.Logger, filters []v1.SubscriptionsAPIFilter) eventfilter.Filter {
	if len(filters) == 0 {
		logger.Debug("no filters provided")
		return NewNoFilter()
	}
	return NewAllFilter(MaterializeFiltersList(logger, filters)...)
}

// MaterialzieFilterList allows any component that supports `SubscriptionsAPIFilter` to process them
func MaterializeFiltersList(logger *zap.Logger, filters []v1.SubscriptionsAPIFilter) []eventfilter.Filter {
	materializedFilters := make([]eventfilter.Filter, 0, len(filters))
	for _, f := range filters {
		f := MaterializeSubscriptionsAPIFilter(logger, f)
		if f == nil {
			logger.Warn("Failed to parse filter. Skipping filter.", zap.Any("filter", f))
			continue
		}
		materializedFilters = append(materializedFilters, f)
	}
	return materializedFilters
}
