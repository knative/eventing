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
package e2e

import (
	"context"
	"github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Fixture interface {
	// Create attempts to instantiate the fixture. It may be called multiple times if earlier attempts return an error.
	Create(context.Context, client.Client) error

	// Verify determines if the fixture was successfully instantiated once. It may be called multiple times if earlier
	// attempts return false or an error.
	Verify(context.Context, client.Client) (bool, error)

	// Debug surfaces context about a failure during Create or Verify.
	Debug(context.Context, client.Client)

	// Teardown removes any state instantiated by Create or Verify.
	Teardown(context.Context, client.Client) error
}

type KnativeFixture struct {
  Object testing.Buildable
}

type CreateFunc func(context.Context, client.Client) error
type VerifyFunc func(context.Context, client.Client) (bool, error)
type DebugFunc func(context.Context, client.Client)
type TeardownFunc func(context.Context, client.Client) error

type FixtureFuncs struct {
	Create CreateFunc
	Verify VerifyFunc
	Debug DebugFunc
	Teardown TeardownFunc
}

//Fixtures:
//	- Broker, Trigger Verify checks status ready
//  - Sendevent Verify checks that pod completed successfully
//	- Logevent Verify checks that the expected message was logged

func (f *KnativeFixture) Create(ctx context.Context, cl client.Client) error {
	obj := f.Object.Build()
	err := cl.Create(ctx, obj)
	if err != nil {
		return err
	}
	return nil
}

var standardCondSet = duckv1alpha1.NewLivingConditionSet()

// TODO the runner should do this with backoff and retry.
// This method only verifies Knative objects
func (f *KnativeFixture) Verify(ctx context.Context, cl client.Client) (bool, error) {
	obj := f.Object.Build()
	u := &unstructured.Unstructured{}
	acc, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}
	if err := cl.Get(ctx, client.ObjectKey{Name: acc.GetName(), Namespace: acc.GetNamespace()}, u); err != nil {
		return false, err
	}

	kr := &duckv1alpha1.KResource{}
	if err := duck.FromUnstructured(u, kr); err != nil {
		return false, err
	}

	if !standardCondSet.Manage(kr.Status).IsHappy() {
		return false, nil
	}

	return true, nil
}

// TODO If Create or Verify return an error, run this method, then Cleanup
func (f *KnativeFixture) Debug(ctx context.Context, cl client.Client) {
}

// TODO if Teardown returns an error, log it and give up (maybe retry for a while?)
func (f *KnativeFixture) Teardownctx context.Context, cl client.Client) error {
	obj := f.Object.Build()
	return cl.Delete(ctx, obj)
}