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
	"testing"
	"time"

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

// IsReady returns a reusable feature.StepFn to assert if a resource is ready
// within the time given.
func IsReady(gvr schema.GroupVersionResource, name string, interval, timeout time.Duration) feature.StepFn {
	return func(ctx context.Context, t *testing.T) {
		env := environment.FromContext(ctx)
		if err := WaitForResourceReady(ctx, env.Namespace(), name, gvr, interval, timeout); err != nil {
			t.Error(gvr, "did not become ready,", err)
		}
	}
}

// IsAddressable tests to see if a resource becomes Addressable within the time
// given.
func IsAddressable(gvr schema.GroupVersionResource, name string, interval, timeout time.Duration) feature.StepFn {
	return func(ctx context.Context, t *testing.T) {
		// Special case Service.
		if gvr.Group == "" && gvr.Version == "v1" && gvr.Resource == "services" {
			log.Printf("[special] %s %s is addressable\n", gvr, name)
			return
		}

		env := environment.FromContext(ctx)
		lastMsg := ""
		like := &duckv1.AddressableType{}
		err := wait.PollImmediate(interval, timeout, func() (bool, error) {
			client := dynamicclient.Get(ctx)

			us, err := client.Resource(gvr).Namespace(env.Namespace()).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					// keep polling
					return false, nil
				}
				return false, err
			}
			obj := like.DeepCopy()
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, obj); err != nil {
				return false, fmt.Errorf("error from DefaultUnstructured.Dynamiconverter. %w", err)
			}
			obj.ResourceVersion = gvr.Version
			obj.APIVersion = gvr.GroupVersion().String()

			if obj.Status.Address == nil || obj.Status.Address.URL == nil {
				msg := fmt.Sprintf("%s %s has no status.address.url, %s", gvr, name, err)
				if msg != lastMsg {
					log.Println(msg)
					lastMsg = msg
				}
				return false, nil
			}

			// Success!
			log.Printf("%s %s is addressable: %s\n", gvr, name, obj.Status.Address.URL)
			return true, nil
		})
		if err != nil {
			t.Error(gvr, "did not become addressable,", err)
		}
	}
}
