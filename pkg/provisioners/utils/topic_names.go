package utils

import (
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

const (
	knativeChannelPrefix = "knative-eventing-channel"
)

func TopicName(channelSeparator, channelNamespace, channelName string) string {
	return topicName(channelSeparator, channelNamespace, channelName)
}

func TopicNameWithUID(channelSeparator, channelName string, channelUID types.UID) string {
	return topicName(channelSeparator, channelName, string(channelUID))
}

func topicName(channelSeparator string, pieces ...string) string {
	parts := make([]string, 0, 1+len(pieces))
	parts = append(parts, knativeChannelPrefix)
	parts = append(parts, pieces...)
	return strings.Join(parts, channelSeparator)
}
