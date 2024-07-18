/*
Copyright 2020 The Knative Authors

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

package tracing

import (
	"fmt"

	"go.opencensus.io/trace"
	"k8s.io/apimachinery/pkg/types"
)

const (
	MessagingSystemAttributeName      = "messaging.system"
	MessagingDestinationAttributeName = "messaging.destination"
	MessagingProtocolAttributeName    = "messaging.protocol"
	MessagingMessageIDAttributeName   = "messaging.message_id"
)

var (
	MessagingSystemAttribute trace.Attribute = trace.StringAttribute(MessagingSystemAttributeName, "knative")
	MessagingProtocolHTTP    trace.Attribute = MessagingProtocolAttribute("HTTP")
)

func MessagingProtocolAttribute(protocol string) trace.Attribute {
	return trace.StringAttribute(MessagingProtocolAttributeName, protocol)
}

func MessagingMessageIDAttribute(ID string) trace.Attribute {
	return trace.StringAttribute(MessagingMessageIDAttributeName, ID)
}

func BrokerMessagingDestination(b types.NamespacedName) string {
	return fmt.Sprintf("broker:%s.%s", b.Name, b.Namespace)
}
func BrokerMessagingDestinationAttribute(b types.NamespacedName) trace.Attribute {
	return trace.StringAttribute(MessagingDestinationAttributeName, BrokerMessagingDestination(b))
}

func TriggerMessagingDestination(t types.NamespacedName) string {
	return fmt.Sprintf("trigger:%s.%s", t.Name, t.Namespace)
}

func TriggerMessagingDestinationAttribute(t types.NamespacedName) trace.Attribute {
	return trace.StringAttribute(MessagingDestinationAttributeName, TriggerMessagingDestination(t))
}
