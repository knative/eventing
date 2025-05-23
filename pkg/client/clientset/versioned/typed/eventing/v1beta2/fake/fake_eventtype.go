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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	gentype "k8s.io/client-go/gentype"
	v1beta2 "knative.dev/eventing/pkg/apis/eventing/v1beta2"
	eventingv1beta2 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1beta2"
)

// fakeEventTypes implements EventTypeInterface
type fakeEventTypes struct {
	*gentype.FakeClientWithList[*v1beta2.EventType, *v1beta2.EventTypeList]
	Fake *FakeEventingV1beta2
}

func newFakeEventTypes(fake *FakeEventingV1beta2, namespace string) eventingv1beta2.EventTypeInterface {
	return &fakeEventTypes{
		gentype.NewFakeClientWithList[*v1beta2.EventType, *v1beta2.EventTypeList](
			fake.Fake,
			namespace,
			v1beta2.SchemeGroupVersion.WithResource("eventtypes"),
			v1beta2.SchemeGroupVersion.WithKind("EventType"),
			func() *v1beta2.EventType { return &v1beta2.EventType{} },
			func() *v1beta2.EventTypeList { return &v1beta2.EventTypeList{} },
			func(dst, src *v1beta2.EventTypeList) { dst.ListMeta = src.ListMeta },
			func(list *v1beta2.EventTypeList) []*v1beta2.EventType { return gentype.ToPointerSlice(list.Items) },
			func(list *v1beta2.EventTypeList, items []*v1beta2.EventType) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
