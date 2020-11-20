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
	"testing"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/injection/clients/dynamicclient"
	pkgtest "knative.dev/pkg/test"

	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/reconciler-test/pkg/environment"
)

func WaitForReadyOrDone(ctx context.Context, ref corev1.ObjectReference, interval, timeout time.Duration) error {
	k := ref.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(k)

	switch gvr.Resource {
	case "jobs":
		err := WaitUntilJobDone(kubeclient.Get(ctx), ref.Namespace, ref.Name, interval, timeout)
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
func WaitForResourceReady(ctx context.Context, namespace, name string, gvr schema.GroupVersionResource, interval, timeout time.Duration) error {
	lastMsg := ""
	like := &duckv1.KResource{}
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		client := dynamicclient.Get(ctx)

		us, err := client.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
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

		ready := obj.Status.GetCondition(apis.ConditionReady)
		if ready != nil && !ready.IsTrue() {
			msg := fmt.Sprintf("%s is not ready, %s: %s", name, ready.Reason, ready.Message)
			if msg != lastMsg {
				log.Println(msg)
				lastMsg = msg
			}
		}

		return ready.IsTrue(), nil
	})
}

// WaitForServiceEndpointsOrFail wraps the utility from pkg and uses the context to extract kubeclient and namespace
func WaitForServiceEndpointsOrFail(ctx context.Context, tb testing.TB, svcName string, numberOfExpectedEndpoints int) {
	if err := pkgtest.WaitForServiceEndpoints(ctx, kubeclient.Get(ctx), svcName, environment.FromContext(ctx).Namespace(), numberOfExpectedEndpoints); err != nil {
		tb.Fatalf("Failed while waiting for %d endpoints in service %s: %+v", numberOfExpectedEndpoints, svcName, errors.WithStack(err))
	}
}

// WaitForPodRunningOrFail wraps the utility from pkg and uses the context to extract kubeclient and namespace
func WaitForPodRunningOrFail(ctx context.Context, tb testing.TB, podName string) {
	if err := pkgtest.WaitForPodRunning(ctx, kubeclient.Get(ctx), podName, environment.FromContext(ctx).Namespace()); err != nil {
		tb.Fatalf("Failed while waiting for pod %s running: %+v", podName, errors.WithStack(err))
	}
}
