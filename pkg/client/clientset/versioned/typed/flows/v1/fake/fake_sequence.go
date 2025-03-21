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
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	flowsv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/flows/v1"
)

// fakeSequences implements SequenceInterface
type fakeSequences struct {
	*gentype.FakeClientWithList[*v1.Sequence, *v1.SequenceList]
	Fake *FakeFlowsV1
}

func newFakeSequences(fake *FakeFlowsV1, namespace string) flowsv1.SequenceInterface {
	return &fakeSequences{
		gentype.NewFakeClientWithList[*v1.Sequence, *v1.SequenceList](
			fake.Fake,
			namespace,
			v1.SchemeGroupVersion.WithResource("sequences"),
			v1.SchemeGroupVersion.WithKind("Sequence"),
			func() *v1.Sequence { return &v1.Sequence{} },
			func() *v1.SequenceList { return &v1.SequenceList{} },
			func(dst, src *v1.SequenceList) { dst.ListMeta = src.ListMeta },
			func(list *v1.SequenceList) []*v1.Sequence { return gentype.ToPointerSlice(list.Items) },
			func(list *v1.SequenceList, items []*v1.Sequence) { list.Items = gentype.FromPointerSlice(items) },
		),
		fake,
	}
}
