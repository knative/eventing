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

// This file contains functions which get property values for
// resources provided by the caller.

package duck

import (
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/test/lib/resources"
)

// GetAddressableURI returns the uri for the given resource that implements Addressable duck-type.
func GetAddressableURI(dynamicClient dynamic.Interface, obj *resources.MetaResource) (url.URL, error) {
	untyped, err := GetGenericObject(dynamicClient, obj, &duckv1.AddressableType{})
	if err != nil {
		return url.URL{}, err
	}

	at := untyped.(*duckv1.AddressableType)

	if at.Status.Address == nil {
		return url.URL{}, fmt.Errorf("addressable does not have an Address: %+v", at)
	}
	if at.Status.Address.URL == nil {
		return url.URL{}, fmt.Errorf("addressable does not have a URL: %+v", at)
	}
	if at.Status.Address.URL.Host == "" {
		return url.URL{}, fmt.Errorf("addressable's URL does not have a Host: %+v", at)
	}
	return url.URL(*at.Status.Address.URL), nil
}

func GetServiceHostname(svc *corev1.Service) string {
	return svc.Name + "." + svc.Namespace
}
