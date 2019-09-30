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

package base

import (
	"k8s.io/client-go/dynamic"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// GetAddressableURL returns the uri for the given resource that implements Addressable duck-type.
func GetAddressableURL(dynamicClient dynamic.Interface, obj *resources.MetaResource) (*apis.URL, error) {
	untyped, err := GetGenericObject(dynamicClient, obj, &duckv1beta1.AddressableType{})
	if err != nil {
		return nil, err
	}

	at := untyped.(*duckv1beta1.AddressableType)
	return at.Status.Address.URL, nil
}
