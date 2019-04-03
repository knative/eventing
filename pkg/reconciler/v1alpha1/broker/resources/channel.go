package resources

import (
	"fmt"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NewChannel creates a new Channel for Broker 'b'.
func NewChannel(b *v1alpha1.Broker, l map[string]string) *v1alpha1.Channel {
	var spec v1alpha1.ChannelSpec
	if b.Spec.ChannelTemplate != nil {
		spec = *b.Spec.ChannelTemplate
	}

	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    b.Namespace,
			GenerateName: fmt.Sprintf("%s-broker-", b.Name),
			Labels:       l,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(b, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Broker",
				}),
			},
		},
		Spec: spec,
	}
}

func NewTriggerChannel(b *v1alpha1.Broker) *v1alpha1.Channel {
	return NewChannel(b, TriggerChannelLabels(b))
}

func NewIngressChannel(b *v1alpha1.Broker) *v1alpha1.Channel {
	return NewChannel(b, IngressChannelLabels(b))
}
