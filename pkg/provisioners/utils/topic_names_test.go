package utils

import (
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

func TestGenerateTopicNameWithDot(t *testing.T) {
	expected := "knative-eventing-channel.channel-namespace.channel-name"
	actual := TopicName(".", "channel-namespace", "channel-name")
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}

func TestGenerateTopicNameWithHyphen(t *testing.T) {
	expected := "knative-eventing-channel-channel-namespace-channel-name"
	actual := TopicName("-", "channel-namespace", "channel-name")
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}

func TestTopicNameWithUID(t *testing.T) {
	expected := "knative-eventing-channel_gcp_78f8428c-15c3-11e9-9905-42010a80018a"
	actual := TopicNameWithUID("_", "gcp", types.UID("78f8428c-15c3-11e9-9905-42010a80018a"))
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}
