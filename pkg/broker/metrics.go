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

package broker

import (
	"go.opencensus.io/tag"
	"knative.dev/eventing/pkg/metrics"
)

const (
	// EventArrivalTime is used to access the metadata stored on a
	// CloudEvent to measure the time difference between when an events is
	// received on a broker and before it is dispatched to the trigger function.
	// The format is an RFC3339 time in string format. For example: 2019-08-26T23:38:17.834384404Z.
	EventArrivalTime = "knativearrivaltime"

	// LabelUniqueName is the label for the unique name per stats_reporter instance.
	LabelUniqueName = "unique_name"

	// LabelContainerName is the label for the immutable name of the container.
	LabelContainerName = metrics.LabelContainerName
)

var (
	ContainerTagKey = tag.MustNewKey(LabelContainerName)
	UniqueTagKey    = tag.MustNewKey(LabelUniqueName)
)
