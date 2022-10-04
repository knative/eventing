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

package testing

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
)

// DefaultingReactor will return a Reactor function that will call SetDefault on newly created Objects
func DefaultingReactor(ctx context.Context) clientgotesting.ReactionFunc {
	return func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if verb := action.GetVerb(); verb != "create" {
			return false, nil, nil
		}

		got := action.(clientgotesting.CreateAction).GetObject()
		obj, ok := got.(apis.Defaultable)
		if !ok {
			return false, nil, nil
		}

		obj.SetDefaults(ctx)
		return false, got, nil
	}
}
