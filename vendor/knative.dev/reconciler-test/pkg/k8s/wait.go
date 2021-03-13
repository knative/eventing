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

package k8s

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/clients/dynamicclient"
	pkgtest "knative.dev/pkg/test"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

// pollTimings will find the the correct timings based on priority:
// - passed timing slice [interval, timeout].
// - values from from context.
// - defaults.
func pollTimings(ctx context.Context, timing []time.Duration) (time.Duration, time.Duration) {
	// Use the passed timing first, but it could be nil or a strange length.
	if len(timing) >= 2 {
		return timing[0], timing[1]
	}

	var interval *time.Duration

	// Use the passed timing if only interval is provided.
	if len(timing) == 1 {
		interval = &timing[0]
	}

	di, timeout := environment.PollTimingsFromContext(ctx)
	if interval == nil {
		interval = &di
	}

	return *interval, timeout
}

// WaitForReadyOrDone will wait for a resource to become ready or succeed.
// Timing is optional but if provided is [interval, timeout].
func WaitForReadyOrDone(ctx context.Context, ref corev1.ObjectReference, timing ...time.Duration) error {
	interval, timeout := pollTimings(ctx, timing)

	k := ref.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(k)

	switch gvr.Resource {
	case "jobs":
		err := WaitUntilJobDone(ctx, kubeclient.Get(ctx), ref.Namespace, ref.Name, interval, timeout)
		if err != nil {
			return err
		}
		return nil

	default:
		err := WaitForResourceReady(ctx, ref.Namespace, ref.Name, gvr, interval, timeout)
		if err != nil {
			return err
		}
	}

	return nil
}

// WaitForResourceReady waits until the specified resource in the given namespace are ready.
// Timing is optional but if provided is [interval, timeout].
func WaitForResourceReady(ctx context.Context, namespace, name string, gvr schema.GroupVersionResource, timing ...time.Duration) error {
	interval, timeout := pollTimings(ctx, timing)

	lastMsg := ""
	like := &duckv1.KResource{}
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		client := dynamicclient.Get(ctx)

		us, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Println(namespace, name, "not found", err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		obj := like.DeepCopy()
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, obj); err != nil {
			log.Fatalf("Error DefaultUnstructuree.Dynamiconverter. %v", err)
		}
		obj.ResourceVersion = gvr.Version
		obj.APIVersion = gvr.GroupVersion().String()

		// Ready type.
		ready := obj.Status.GetCondition(apis.ConditionReady)
		if ready != nil {
			// Succeeded type.
			ready = obj.Status.GetCondition(apis.ConditionSucceeded)
		}
		// Test Ready or Succeeded.
		if ready != nil {
			if !ready.IsTrue() {
				msg := fmt.Sprintf("%s is not %s, %s: %s", name, ready.Type, ready.Reason, ready.Message)
				if msg != lastMsg {
					log.Println(msg)
					lastMsg = msg
				}
			}

			log.Printf("%s is %s, %s: %s\n", name, ready.Type, ready.Reason, ready.Message)
			return ready.IsTrue(), nil
		}

		// Last resort, look at all conditions.
		// As a side-effect of this test,
		//   if a resource has no conditions, then it is ready.
		allReady := true
		for _, c := range obj.Status.Conditions {
			if !c.IsTrue() {
				msg := fmt.Sprintf("%s is not %s, %s: %s", name, c.Type, c.Reason, c.Message)
				if msg != lastMsg {
					log.Println(msg)
					lastMsg = msg
				}
				allReady = false
				break
			}
		}
		return allReady, nil
	})
}

// WaitForServiceEndpointsOrFail wraps the utility from pkg and uses the context to extract kubeclient and namespace
func WaitForServiceEndpointsOrFail(ctx context.Context, t feature.T, svcName string, numberOfExpectedEndpoints int) {
	if err := pkgtest.WaitForServiceEndpoints(ctx, kubeclient.Get(ctx), svcName, environment.FromContext(ctx).Namespace(), numberOfExpectedEndpoints); err != nil {
		t.Fatalf("Failed while waiting for %d endpoints in service %s: %+v", numberOfExpectedEndpoints, svcName, errors.WithStack(err))
	}
}

// WaitForPodRunningOrFail wraps the utility from pkg and uses the context to extract kubeclient and namespace
func WaitForPodRunningOrFail(ctx context.Context, t feature.T, podName string) {
	if err := pkgtest.WaitForPodRunning(ctx, kubeclient.Get(ctx), podName, environment.FromContext(ctx).Namespace()); err != nil {
		t.Fatalf("Failed while waiting for pod %s running: %+v", podName, errors.WithStack(err))
	}
}

// WaitForAddress waits until a resource has an address.
// Timing is optional but if provided is [interval, timeout].
func WaitForAddress(ctx context.Context, gvr schema.GroupVersionResource, name string, timing ...time.Duration) (*apis.URL, error) {
	interval, timeout := pollTimings(ctx, timing)

	var addr *apis.URL
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		var err error
		addr, err = Address(ctx, gvr, name)
		if err == nil && addr == nil {
			// keep polling
			return false, nil
		}
		if err != nil {
			if apierrors.IsNotFound(err) {
				// keep polling
				return false, nil
			}
			// seems fatal.
			return false, err
		}
		// success!
		return true, nil
	})
	return addr, err
}
