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

package knconf

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

func HasReadyInConditions(_ context.Context, t feature.T, status duckv1.Status) {
	found := []string(nil)
	for _, c := range status.Conditions {
		if c.Type == "Ready" {
			// Success!
			return
		}
		found = append(found, string(c.Type))
	}
	t.Errorf(`does not have "Ready" condition, has: [%s]`, strings.Join(found, ","))
}

func KResourceHasObservedGeneration(gvr schema.GroupVersionResource, name string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		ri := dynamicclient.Get(ctx).Resource(gvr).Namespace(env.Namespace())

		get := func() (*duckv1.KResource, error) {
			obj, err := ri.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			kr := new(duckv1.KResource)
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, kr); err != nil {
				return nil, err
			}
			return kr, nil
		}

		var kr *duckv1.KResource
		var err error

		interval, timeout := environment.PollTimingsFromContext(ctx)
		err = wait.PollImmediate(interval, timeout, func() (bool, error) {
			kr, err = get()
			if err != nil {
				// break out if not a "not found" error.
				if !apierrors.IsNotFound(err) {
					return false, err
				}
				// Keep polling
				return false, nil
			}
			if kr.Status.ObservedGeneration == kr.Generation {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			t.Errorf("unable to get a reconciled resource (status.observedGeneration != 0)")
		}
	}
}

func KResourceHasReadyInConditions(gvr schema.GroupVersionResource, name string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		ri := dynamicclient.Get(ctx).Resource(gvr).Namespace(env.Namespace())

		get := func() (*duckv1.KResource, error) {
			obj, err := ri.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			kr := new(duckv1.KResource)
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, kr); err != nil {
				return nil, err
			}
			return kr, nil
		}

		var kr *duckv1.KResource
		var err error

		interval, timeout := environment.PollTimingsFromContext(ctx)
		err = wait.PollImmediate(interval, timeout, func() (bool, error) {
			kr, err = get()
			if err != nil {
				// break out if not a "not found" error.
				if !apierrors.IsNotFound(err) {
					return false, err
				}
				// Keep polling
				return false, nil
			}
			if kr.Status.ObservedGeneration == kr.Generation {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			t.Errorf("unable to get a reconciled resource (status.observedGeneration != 0)")
		}

		HasReadyInConditions(ctx, t, kr.Status)
	}
}
