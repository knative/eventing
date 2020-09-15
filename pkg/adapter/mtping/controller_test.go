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

package mtping

import (
	"context"
	"testing"

	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/sources/v1beta1/pingsource/fake"

	"knative.dev/eventing/pkg/apis/sources/v1beta1"
)

type dummyAdapter struct{}

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	c := NewController(ctx, &dummyAdapter{})

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

func (dummyAdapter) Start(ctx context.Context) error {
	return nil
}

func (dummyAdapter) Update(ctx context.Context, source *v1beta1.PingSource) {
}

func (dummyAdapter) Remove(ctx context.Context, source *v1beta1.PingSource) {
}
