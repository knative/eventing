/*
Copyright 2022 The Knative Authors

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

package feature

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/clients/dynamicclient"
)

func LogReferences(refs ...corev1.ObjectReference) StepFn {
	return func(ctx context.Context, t T) {
		for _, ref := range refs {
			logReference(ref)(ctx, t)
		}
	}
}

func logReference(ref corev1.ObjectReference) StepFn {
	return func(ctx context.Context, t T) {
		dc := dynamicclient.Get(ctx)

		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			t.Fatalf("Could not parse GroupVersion for %+v", ref.APIVersion)
		}

		resource := apis.KindToResource(gv.WithKind(ref.Kind))

		resourceStr := fmt.Sprintf("Resource %+v %s/%s", resource, ref.Namespace, ref.Name)

		r, err := dc.Resource(resource).
			Namespace(ref.Namespace).
			Get(ctx, ref.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			t.Logf("%s not found: %v\n", resourceStr, err)
			return
		}
		if err != nil {
			t.Logf("%s: %v\n", resourceStr, err)
			return
		}

		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			t.Logf("Failed to marshal %s: %v\n", resourceStr, err)
			return
		}

		// Get events for the given resource
		events, _ := kubeclient.Get(ctx).EventsV1().
			Events(ref.Namespace).
			List(ctx, metav1.ListOptions{
				TypeMeta: metav1.TypeMeta{
					Kind:       ref.Kind,
					APIVersion: ref.APIVersion,
				},
				FieldSelector: fmt.Sprintf("involvedObject.name=%s", ref.Name),
				Limit:         50,
			})
		eBytes, _ := json.MarshalIndent(events, "", "  ")

		t.Logf("%s\n%s\nEvents:\n%s\n", resourceStr, string(b), string(eBytes))

		// Recursively log owners
		for _, or := range r.GetOwnerReferences() {
			t.Logf("Logging owner for %s\n", resourceStr)
			logReference(corev1.ObjectReference{
				Kind:       or.Kind,
				Namespace:  r.GetNamespace(),
				Name:       or.Name,
				APIVersion: or.APIVersion,
			})
		}
	}
}
