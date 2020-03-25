/*
Copyright 2020 The Knative Authors

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

package resources

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/tracker"
)

var subjectGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")

func MakeSinkBinding(source *v1alpha1.ContainerSource) *v1alpha1.SinkBinding {
	subjectAPIVersion, subjectKind := subjectGVK.ToAPIVersionAndKind()

	sb := &v1alpha1.SinkBinding{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
			Name:      kmeta.ChildName(source.Name, "-containersource" /*suffix*/),
			Namespace: source.Namespace,
		},
		Spec: v1alpha1.SinkBindingSpec{
			SourceSpec: source.Spec.SourceSpec,
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: subjectAPIVersion,
					Kind:       subjectKind,
					Selector: &metav1.LabelSelector{
						MatchLabels: Labels(source.Name),
					},
				},
			},
		},
	}
	return sb
}
