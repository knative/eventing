package resources

import (
	"fmt"
	"github.com/knative/eventing/contrib/kafka/pkg/apis/messaging/v1alpha1"
)

func MakeTopicName(channel *v1alpha1.KafkaChannel) string {
	return fmt.Sprintf("%s.%s", channel.Namespace, channel.Name)
}
