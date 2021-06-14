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

package scheme

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var Eventing = &runtime.SchemeBuilder{
	SubscriberToScheme,
	TestSourceToScheme,
}

var Serving = runtime.SchemeBuilder{
	ServingServiceToScheme,
}

func SubscriberToScheme(scheme *runtime.Scheme) error {
	gv := schema.GroupVersion{Group: "messaging.knative.dev", Version: "v1"}
	scheme.AddKnownTypeWithName(gv.WithKind("Subscriber"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gv.WithKind("SubscriberList"), &unstructured.UnstructuredList{})
	return nil
}

func TestSourceToScheme(scheme *runtime.Scheme) error {
	gv := schema.GroupVersion{Group: "testing.sources.knative.dev", Version: "v1"}
	scheme.AddKnownTypeWithName(gv.WithKind("TestSource"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gv.WithKind("TestSourceList"), &unstructured.UnstructuredList{})
	return nil
}

func ServingServiceToScheme(scheme *runtime.Scheme) error {
	gv := schema.GroupVersion{Group: "serving.knative.dev", Version: "v1"}
	scheme.AddKnownTypeWithName(gv.WithKind("Service"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gv.WithKind("ServiceList"), &unstructured.UnstructuredList{})
	scheme.AddKnownTypes(gv, &metav1.Status{})
	metav1.AddToGroupVersion(scheme, gv)
	return nil
}
