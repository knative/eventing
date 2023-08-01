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

package addressable

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/k8s"
)

type ValidateAddress func(addressable *duckv1.Addressable) error

// Address returns a broker's address.
func Address(ctx context.Context, gvr schema.GroupVersionResource, name string, timings ...time.Duration) (*duckv1.Addressable, error) {
	interval, timeout := k8s.PollTimings(ctx, timings)
	var addr *duckv1.Addressable
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		var err error
		addr, err = k8s.Address(ctx, gvr, name)
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

func AssertHTTPSAddress(addr *duckv1.Addressable) error {
	if addr.URL.Scheme != "https" {
		return fmt.Errorf("address is not HTTPS: %#v", addr)
	}
	return nil
}
