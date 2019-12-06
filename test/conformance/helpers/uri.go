/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helpers

import (
	"fmt"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
)

func brokerIngressHost(domain string, broker v1alpha1.Broker) string {
	return fmt.Sprintf("%s-broker.%s.svc.%s", broker.Name, broker.Namespace, domain)
}

func channelHost(domain, namespace, chanName string) string {
	return fmt.Sprintf("%s-kn-channel.%s.svc.%s", chanName, namespace, domain)
}

func brokerTriggerChannelHost(domain string, broker v1alpha1.Broker) string {
	return channelHost(domain, broker.Namespace, fmt.Sprintf("%s-kne-trigger", broker.Name))
}

func brokerFilterHost(domain string, broker v1alpha1.Broker) string {
	return fmt.Sprintf("%s-broker-filter.%s.svc.%s", broker.Name, broker.Namespace, domain)
}

func triggerPath(trigger v1alpha1.Trigger) string {
	return fmt.Sprintf("/triggers/%s/%s/%s", trigger.Namespace, trigger.Name, trigger.UID)
}

func k8sServiceHost(domain, namespace, svcName string) string {
	return fmt.Sprintf("%s.%s.svc.%s", svcName, namespace, domain)
}
