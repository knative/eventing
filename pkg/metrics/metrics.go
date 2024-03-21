/*
Copyright 2021 The Knative Authors

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

package metrics

import (
	"knative.dev/pkg/metrics/metricskey"
)

const (
	// ResourceTypeKnativeTrigger is the Stackdriver resource type for Knative Triggers.
	ResourceTypeKnativeTrigger = "knative_trigger"

	// ResourceTypeKnativeBroker is the Stackdriver resource type for Knative Brokers.
	ResourceTypeKnativeBroker = "knative_broker"

	// ResourceTypeKnativeSource is the Stackdriver resource type for Knative Sources.
	ResourceTypeKnativeSource = "knative_source"

	// LabelName is the label for the name of the resource.
	LabelName = "name"

	// LabelResourceGroup is the name of the resource CRD.
	LabelResourceGroup = "resource_group"

	// LabelTriggerName is the label for the name of the Trigger.
	LabelTriggerName = "trigger_name"

	// LabelBrokerName is the label for the name of the Broker.
	LabelBrokerName = "broker_name"

	// LabelEventType is the label for the name of the event type.
	LabelEventType = "event_type"

	// LabelEventType is the label for the name of the event type.
	LabelEventScheme = "event_scheme"

	// LabelEventSource is the label for the name of the event source.
	LabelEventSource = "event_source"

	// LabelFilterType is the label for the Trigger filter attribute "type".
	LabelFilterType = "filter_type"

	// LabelResponseCode is the label for the HTTP response status code.
	LabelResponseCode = metricskey.LabelResponseCode

	// LabelResponseCodeClass is the label for the HTTP response status code class. For example, "2xx", "3xx", etc.
	LabelResponseCodeClass = metricskey.LabelResponseCodeClass

	// LabelNamespaceName is the label for immutable name of the namespace that the service is deployed
	LabelNamespaceName = metricskey.LabelNamespaceName

	// ContainerName is the container for which the metric is reported.
	LabelContainerName = metricskey.ContainerName

	// LabelResponseError is the label for client error. For HTTP, A non-2xx status code doesn't cause an error.
	LabelResponseError = metricskey.LabelResponseError

	// LabelResponseTimeout is the label timeout.
	LabelResponseTimeout = metricskey.LabelResponseTimeout
)
