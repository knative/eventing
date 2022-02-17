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
	"encoding/json"
	"fmt"
	"strings"
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

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

// PollTimings will find the correct timings based on priority:
// - passed timing slice [interval, timeout].
// - values from from context.
// - defaults.
func PollTimings(ctx context.Context, timings []time.Duration) (time.Duration /*interval*/, time.Duration /*timeout*/) {
	// Use the passed timing first, but it could be nil or a strange length.
	if len(timings) >= 2 {
		return timings[0], timings[1]
	}

	var interval *time.Duration

	// Use the passed timings if only interval is provided.
	if len(timings) == 1 {
		interval = &timings[0]
	}

	di, timeout := environment.PollTimingsFromContext(ctx)
	if interval == nil {
		interval = &di
	}

	return *interval, timeout
}

// WaitForReadyOrDone will wait for a resource to become ready or succeed.
// Timing is optional but if provided is [interval, timeout].
func WaitForReadyOrDone(ctx context.Context, t feature.T, ref corev1.ObjectReference, timing ...time.Duration) error {
	interval, timeout := PollTimings(ctx, timing)

	k := ref.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(k)

	switch gvr.Resource {
	case "jobs":
		err := WaitUntilJobDone(ctx, t, kubeclient.Get(ctx), ref.Namespace, ref.Name, interval, timeout)
		if err != nil {
			return err
		}
		return nil

	default:
		err := WaitForResourceReady(ctx, t, ref.Namespace, ref.Name, gvr, interval, timeout)
		if err != nil {
			return err
		}
	}

	return nil
}

// WaitForResourceReady waits until the specified resource in the given namespace are ready.
// Timing is optional but if provided is [interval, timeout].
func WaitForResourceReady(ctx context.Context, t feature.T, namespace, name string, gvr schema.GroupVersionResource, timing ...time.Duration) error {
	return WaitForResourceCondition(ctx, t, namespace, name, gvr, isReady(t, name), timing...)
}

// WaitForResourceNotReady waits until the specified resource in the given namespace is not ready.
// Only the top level ready condition is considered (internal `happy` condition of knative.dev/pkg).
// Timing is optional but if provided is [interval, timeout].
func WaitForResourceNotReady(ctx context.Context, t feature.T, namespace, name string, gvr schema.GroupVersionResource, timing ...time.Duration) error {
	return WaitForResourceCondition(ctx, t, namespace, name, gvr, isNotReady(t, name), timing...)
}

// ConditionFunc is a function that determines whether a condition on a resource is satisfied.
type ConditionFunc func(resource duckv1.KResource) bool

// WaitForResourceCondition waits until the specified resource in the given namespace satisfies a given condition.
// Timing is optional but if provided is [interval, timeout].
func WaitForResourceCondition(ctx context.Context, t feature.T, namespace, name string, gvr schema.GroupVersionResource, condition ConditionFunc, timing ...time.Duration) error {
	interval, timeout := PollTimings(ctx, timing)

	like := &duckv1.KResource{}
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		client := dynamicclient.Get(ctx)

		us, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Log(namespace, name, "not found", err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		obj := like.DeepCopy()
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, obj); err != nil {
			t.Fatalf("Error DefaultUnstructured.Dynamiconverter. %v", err)
		}
		obj.ResourceVersion = gvr.Version
		obj.APIVersion = gvr.GroupVersion().String()

		// First see if the resource has conditions.
		if len(obj.Status.Conditions) == 0 {
			t.Log("Resource has no conditions")
			return false, nil // keep polling
		}

		// Verify condition.
		return condition(*obj), nil
	})
}

func isReady(t feature.T, name string) ConditionFunc {
	lastMsg := ""
	return func(obj duckv1.KResource) bool {
		if obj.Generation != obj.GetStatus().ObservedGeneration {
			return false
		}
		ready := readyCondition(obj)
		if ready != nil {
			if !ready.IsTrue() {
				msg := fmt.Sprintf("%s is not %s\n\nResource: %s\n", name, ready.Type, resource(obj))
				if msg != lastMsg {
					t.Log(msg)
					lastMsg = msg
				}
			}

			t.Logf("%s is %s, %s: %s\n", name, ready.Type, ready.Reason, ready.Message)
			return ready.IsTrue()
		}

		// Last resort, look at all conditions.
		// As a side-effect of this test,
		//   if a resource has no conditions, then it is ready.
		allReady := true
		for _, c := range obj.Status.Conditions {
			if !c.IsTrue() {
				msg := fmt.Sprintf("%s is not %s, %s: %s\n\nResource: %s\n", name, c.Type, c.Reason, c.Message, resource(obj))
				if msg != lastMsg {
					t.Log(msg)
					lastMsg = msg
				}
				allReady = false
				break
			}
		}
		return allReady
	}
}

func isNotReady(t feature.T, name string) ConditionFunc {
	lastMsg := ""
	return func(obj duckv1.KResource) bool {
		ready := readyCondition(obj)
		if ready == nil {
			msg := fmt.Sprintf("%s hasn't any of %s or %s conditions\n\nResource: %s\n", name, apis.ConditionReady, apis.ConditionSucceeded, resource(obj))
			if msg != lastMsg {
				t.Log(msg)
				lastMsg = msg
			}
			return false
		}
		t.Logf("%s is %s, %s: %s\n\nResource: %s\n", name, ready.Type, ready.Reason, ready.Message, resource(obj))
		return ready.IsFalse()
	}
}

func resource(obj duckv1.KResource) string {
	resource, err := json.MarshalIndent(obj, " ", " ")
	if err != nil {
		resource = []byte(err.Error())
	}
	return string(resource)
}

// readyCondition returns Ready or Succeeded condition.
func readyCondition(obj duckv1.KResource) *apis.Condition {
	// Ready type.
	ready := obj.Status.GetCondition(apis.ConditionReady)
	if ready != nil {
		return ready
	}
	// Succeeded type.
	return obj.Status.GetCondition(apis.ConditionSucceeded)
}

// WaitForServiceEndpointsOrFail polls the status of the specified Service
// every interval until number of service endpoints >= numOfEndpoints.
func WaitForServiceEndpointsOrFail(ctx context.Context, t feature.T, svcName string, numberOfExpectedEndpoints int) {
	endpointsService := kubeclient.Get(ctx).CoreV1().Endpoints(environment.FromContext(ctx).Namespace())
	interval, timeout := PollTimings(ctx, nil)
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		endpoint, err := endpointsService.Get(ctx, svcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return countEndpointsNum(endpoint) >= numberOfExpectedEndpoints, nil
	})
	if err != nil {
		t.Fatalf("Failed while waiting for %d endpoints in service %s: %+v", numberOfExpectedEndpoints, svcName, errors.WithStack(err))
	}
}

// WaitForPodRunningOrFail waits for the given pod to be in running state.
func WaitForPodRunningOrFail(ctx context.Context, t feature.T, podName string) {
	ns := environment.FromContext(ctx).Namespace()
	podClient := kubeclient.Get(ctx).CoreV1().Pods(ns)
	p := podClient
	interval, timeout := PollTimings(ctx, nil)
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		p, err := p.Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		isRunning := podRunning(p)

		if !isRunning {
			t.Log("Pod %s/%s is not running, dumping the pod object we got:", ns, podName)
			b, _ := json.MarshalIndent(p, "", " ")
			t.Log(string(b))
		}

		return isRunning, nil
	})
	if err != nil {
		sb := strings.Builder{}
		if p, err := podClient.Get(ctx, podName, metav1.GetOptions{}); err != nil {
			sb.WriteString(err.Error())
			sb.WriteString("\n")
		} else {
			for _, c := range p.Spec.Containers {
				if b, err := PodLogs(ctx, podName, c.Name, environment.FromContext(ctx).Namespace()); err != nil {
					sb.WriteString(err.Error())
				} else {
					sb.Write(b)
				}
				sb.WriteString("\n")
			}
		}
		t.Fatalf("Failed while waiting for pod %s running: %+v\n%s\n", podName, errors.WithStack(err), sb.String())
	}
}

// PodLogs returns Pod logs for given Pod and Container in the namespace
func PodLogs(ctx context.Context, podName, containerName, namespace string) ([]byte, error) {
	podClient := kubeclient.Get(ctx).CoreV1().Pods(namespace)
	podList, err := podClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range podList.Items {
		// Pods are big, so avoid copying.
		pod := &podList.Items[i]
		if strings.Contains(pod.Name, podName) {
			result := podClient.GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: containerName,
			}).Do(ctx)
			return result.Raw()
		}
	}
	return nil, fmt.Errorf("could not find logs for %s/%s:%s", namespace, podName, containerName)
}

// WaitForAddress waits until a resource has an address.
// Timing is optional but if provided is [interval, timeout].
func WaitForAddress(ctx context.Context, gvr schema.GroupVersionResource, name string, timing ...time.Duration) (*apis.URL, error) {
	interval, timeout := PollTimings(ctx, timing)

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

func countEndpointsNum(e *corev1.Endpoints) int {
	if e == nil || e.Subsets == nil {
		return 0
	}
	num := 0
	for _, sub := range e.Subsets {
		num += len(sub.Addresses)
	}
	return num
}

// podRunning will check the status conditions of the pod and return true if it's Running.
func podRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodSucceeded
}
