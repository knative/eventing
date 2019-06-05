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

package apiserversource

//
//import (
//	"context"
//	"github.com/knative/pkg/configmap"
//	"k8s.io/apimachinery/pkg/runtime"
//	"testing"
//
//	"github.com/knative/pkg/logging"
//	logtesting "github.com/knative/pkg/logging/testing"
//
//	fakeeventingclient "github.com/knative/eventing/pkg/client/injection/client/fake"
//	fakedynamicclient "github.com/knative/pkg/injection/clients/dynamicclient/fake"
//	fakekubeclient "github.com/knative/pkg/injection/clients/kubeclient/fake"
//
//	// Fake injection informers
//	_ "github.com/knative/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype/fake"
//	fakeapiserversource "github.com/knative/eventing/pkg/client/injection/informers/sources/v1alpha1/apiserversource/fake"
//	_ "github.com/knative/pkg/injection/informers/kubeinformers/appsv1/deployment/fake"
//)
//
//func TestNew(t *testing.T) {
//	defer logtesting.ClearAll()
//
//	ctx := context.Background()
//	ctx = logging.WithLogger(ctx, logtesting.TestLogger(t))
//	ctx, _ = fakekubeclient.With(ctx)
//	ctx, _ = fakeeventingclient.With(ctx)
//	ctx, _ = fakedynamicclient.With(ctx, runtime.NewScheme())
//
//	fakeapiserversource.Get(ctx)
//
//	c := NewController(ctx, configmap.NewFixedWatcher())
//
//	if c == nil {
//		t.Fatal("Expected NewController to return a non-nil value")
//	}
//}
