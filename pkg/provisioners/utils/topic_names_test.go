package utils

import (
	"testing"
)

func TestGenerateTopicName(t *testing.T) {
	expected := "knative-eventing-channel_channel-namespace.channel-name"
	actual := TopicName("channel-namespace", "channel-name")
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}
