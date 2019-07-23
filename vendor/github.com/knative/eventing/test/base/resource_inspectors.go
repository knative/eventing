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
	"fmt"

	"github.com/knative/eventing/test/base/resources"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"k8s.io/client-go/dynamic"
)

// GetAddressableURI returns the uri for the given resource that implements Addressable duck-type.
func GetAddressableURI(dynamicClient dynamic.Interface, obj *resources.MetaResource) (string, error) {
	untyped, err := GetGenericObject(dynamicClient, obj, &duckv1alpha1.AddressableType{})
	if err != nil {
		return "", err
	}

	at := untyped.(*duckv1alpha1.AddressableType)
	uri := fmt.Sprintf("http://%s", at.Status.Address.Hostname)
	return uri, nil
}
