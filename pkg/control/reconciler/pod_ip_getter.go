/*
Copyright 2021 The Knative Authors

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

package reconciler

import (
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/listers/core/v1"
)

type PodIpGetter struct {
	Lister v1.PodLister
}

func (ipGetter PodIpGetter) GetAllPodsIp(namespace string, selector labels.Selector) ([]string, error) {
	pods, err := ipGetter.Lister.Pods(namespace).List(selector)
	if err != nil {
		return nil, err
	}

	ips := make([]string, 0, len(pods))

	for _, p := range pods {
		if p.Status.PodIP != "" {
			ips = append(ips, p.Status.PodIP)
		}
	}
	return ips, nil
}
