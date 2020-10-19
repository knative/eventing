/*
Copyright 2019 The Knative Authors

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

package duck

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
)

const (
	interval = 1 * time.Second
	timeout  = 2 * time.Minute
)

// WaitForResourceReady polls the status of the MetaResource from client
// every interval until isResourceReady returns `true` indicating
// it is done, returns an error or timeout.
func WaitForResourceReady(dynamicClient dynamic.Interface, obj *resources.MetaResource) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		return checkResourceReady(dynamicClient, obj)
	})
}

func checkResourceReady(dynamicClient dynamic.Interface, obj *resources.MetaResource) (bool, error) {
	untyped, err := GetGenericObject(dynamicClient, obj, &kresourceForReadiness{})
	return isResourceReady(untyped, err)
}

// WaitForResourcesReady waits until all the specified resources in the given namespace are ready.
func WaitForResourcesReady(dynamicClient dynamic.Interface, objList *resources.MetaResourceList) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		untypeds, err := GetGenericObjectList(dynamicClient, objList, &kresourceForReadiness{})
		for _, untyped := range untypeds {
			if isReady, err := isResourceReady(untyped, err); !isReady || err != nil {
				return isReady, err
			}
		}
		return true, nil
	})
}

// isResourceReady leverage duck-type to check if the given obj is in ready state
func isResourceReady(obj runtime.Object, err error) (bool, error) {
	if k8serrors.IsNotFound(err) {
		// Return false as we are not done yet.
		// We swallow the error to keep on polling.
		// It should only happen if we wait for the auto-created resources, like default Broker.
		return false, nil
	} else if err != nil {
		// Return error to stop the polling.
		return false, err
	}
	kr, iskresourceForReadiness := obj.(*kresourceForReadiness)
	if !iskresourceForReadiness {
		return false, fmt.Errorf("obj is not *kresourceForReadiness, actually %T", obj)
	}

	if isPod(kr) {
		return isPodRunning(kr), nil
	}

	ready := kr.Status.GetCondition(apis.ConditionReady)
	return ready != nil && ready.IsTrue(), nil
}

func isPod(obj runtime.Object) bool {
	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	return apiVersion == "v1" && kind == "Pod"
}

// isPodRunning will check the status conditions of the pod and return true if it's Running or Succeeded.
func isPodRunning(pod *kresourceForReadiness) bool {
	return pod.Status.Phase == string(corev1.PodRunning) ||
		pod.Status.Phase == string(corev1.PodSucceeded)
}
