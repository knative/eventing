package tracing

import (
	"fmt"

	"go.opencensus.io/trace"
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
