package utils

import (
	"fmt"
)

func TopicName(channelNamespace, channelName string) string {
	return fmt.Sprintf("knative-eventing-channel_%s.%s", channelNamespace, channelName)
}
