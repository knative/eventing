package utils

import (
	"fmt"
)

func TopicName(channelSeparator, channelNamespace, channelName string) string {
	return fmt.Sprintf("knative-eventing-channel%s%s%s%s", channelSeparator, channelNamespace, channelSeparator, channelName)
}
