/*
Copyright 2022 The Knative Authors

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
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"knative.dev/eventing/pkg/eventfilter"
)

type noFilter struct{}

func NewNoFilter() eventfilter.Filter {
	return noFilter{}
}

func (filter noFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	return eventfilter.NoFilter
}

func (filter noFilter) Cleanup() {}
