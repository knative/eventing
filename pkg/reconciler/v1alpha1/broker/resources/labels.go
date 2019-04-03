package resources

import "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"

// TriggerChannelLabels are all the labels placed on the Trigger Channel for the given Broker. This
// should only be used by Broker and Trigger code.
func TriggerChannelLabels(b *v1alpha1.Broker) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":           b.Name,
		"eventing.knative.dev/brokerEverything": "true",
	}
}

// IngressChannelLabels are all the labels placed on the Ingress Channel for the given Broker. This
// should only be used by Broker and Trigger code.
func IngressChannelLabels(b *v1alpha1.Broker) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":        b.Name,
		"eventing.knative.dev/brokerIngress": "true",
	}
}

func IngressSubscriptionLabels(b *v1alpha1.Broker) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":        b.Name,
		"eventing.knative.dev/brokerIngress": "true",
	}
}
