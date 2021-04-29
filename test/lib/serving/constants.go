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

package serving

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Kind for Knative resources.
const (
	KServiceKind = "Service"
	APIVersion   = "serving.knative.dev/v1"
)

var (
	// KServicesGVR is GroupVersionResource for Knative Service
	KServicesGVR = schema.GroupVersionResource{
		Group:    "serving.knative.dev",
		Version:  "v1",
		Resource: "services",
	}
	// KServiceType is type of Knative Service
	KServiceType = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: KServicesGVR.GroupVersion().String(),
	}
)
