/*
Copyright 2025 The Knative Authors

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

package observability

import (
	ceo11y "github.com/cloudevents/sdk-go/v2/observability"
	"knative.dev/pkg/observability/attributekey"
)

const (
	KnativeMessagingSystem = "knative"
)

var (
	// required attributes for otel messaging span semantics: https://opentelemetry.io/docs/specs/semconv/messaging/messaging-spans/#conventions
	MessagingSystem              = attributekey.String("messaging.system")
	MessagingOperationName       = attributekey.String("messaging.operation.name")
	MessagingDestinationName     = attributekey.String("messaging.destination.name")
	MessagingDestinationTemplate = attributekey.String("messaging.destination.tempate")

	// attributes relating to the source
	SourceName          = attributekey.String("kn.source.name")
	SourceNamespace     = attributekey.String("kn.source.namespace")
	SourceResourceGroup = attributekey.String("kn.source.resourcegroup")

	// attributes relating to the broker
	BrokerName      = attributekey.String("kn.broker.name")
	BrokerNamespace = attributekey.String("kn.broker.namespace")

	// attributes relating to the channel
	ChannelName      = attributekey.String("kn.channel.name")
	ChannelNamespace = attributekey.String("kn.channel.namespace")

	// attributes relating to the broker
	SinkName      = attributekey.String("kn.sink.name")
	SinkNamespace = attributekey.String("kn.sink.namespace")
	SinkKind      = attributekey.String("kn.sink.kind")

	// attributes relating to the cloudevent (not including the ID for cardinality reasons)
	CloudEventType            = attributekey.String(ceo11y.TypeAttr)
	CloudEventSource          = attributekey.String(ceo11y.SourceAttr)
	CloudEventSubject         = attributekey.String(ceo11y.SubjectAttr)
	CloudEventDataContenttype = attributekey.String(ceo11y.DatacontenttypeAttr)
	CloudEventSpecVersion     = attributekey.String(ceo11y.SpecversionAttr)

	// attributes relating to the request
	RequestScheme = attributekey.String("url.scheme")
)
