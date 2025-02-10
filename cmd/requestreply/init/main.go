package v1alpha1

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func main() {
	podIP := os.Getenv("POD_IP")
	port := os.Getenv("PORT")
	triggerUrl, err := apis.ParseURL(fmt.Sprintf("http://%s:%s", podIP, port))
	if err != nil {
		panic(err.Error())
	}

	t, err := client.Triggers(triggerNamespace).Create(context.Background(), &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      triggerName,
			Namespace: triggerNamespace,
		},
		Spec: eventingv1.TriggerSpec{
			Broker: brokerName,
			Filters: []eventingv1.SubscriptionsAPIFilter{
				{CESQL: "the correct CESQL filter here"},
			},
			Subscriber: duckv1.Destination{
				URI: triggerUrl,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Trigger created:", t.Name)
}
