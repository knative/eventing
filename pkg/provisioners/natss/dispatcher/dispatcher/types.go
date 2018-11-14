package dispatcher

import (
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"

	"fmt"
)

// subscription.Ref.Name + "." + subscription.Ref.Namespace

type subscriptionReference struct {
	Name      string
	Namespace string
	SubscriberURI string
	ReplyURI string
}

func newSubscriptionReference(spec eventingduck.ChannelSubscriberSpec) subscriptionReference {
	return subscriptionReference{
		Name:          spec.Ref.Name,
		Namespace:     spec.Ref.Namespace,
		SubscriberURI: spec.SubscriberURI,
		ReplyURI:      spec.ReplyURI,
	}
}

func (r *subscriptionReference) String() string {
	return fmt.Sprintf("%s.%s", r.Name, r.Namespace)
}

