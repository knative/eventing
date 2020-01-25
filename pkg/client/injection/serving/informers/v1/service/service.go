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

package service

import (
	"context"
	"strings"

	"k8s.io/client-go/discovery"

	informerv1 "knative.dev/eventing/pkg/client/injection/serving/informers/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	controller "knative.dev/pkg/controller"
	injection "knative.dev/pkg/injection"
	logging "knative.dev/pkg/logging"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	v1 "knative.dev/serving/pkg/client/informers/externalversions/serving/v1"
	factory "knative.dev/serving/pkg/client/injection/informers/factory"
)

func init() {
	injection.Default.RegisterInformer(withInformer)
}

// Key is used for associating the Informer inside the context.Context.
type Key struct{}

func withInformer(ctx context.Context) (context.Context, controller.Informer) {
	kc := kubeclient.Get(ctx)
	// Register an empty informer if the serving API is not available.
	if err := discovery.ServerSupportsVersion(kc.Discovery(), servingv1.SchemeGroupVersion); err != nil {
		if strings.Contains(err.Error(), "server does not support API version") {
			var inf informerv1.EmptyableServiceInformer
			inf = &emptyableImpl{}
			return context.WithValue(ctx, Key{}, inf), inf
		}
		logging.FromContext(ctx).Panic(
			"Failed to check API version", servingv1.SchemeGroupVersion)
	}

	f := factory.Get(ctx)
	var inf informerv1.EmptyableServiceInformer
	inf = &emptyableImpl{internal: f.Serving().V1().Services()}
	return context.WithValue(ctx, Key{}, inf), inf
}

// Get extracts the typed informer from the context.
func Get(ctx context.Context) informerv1.EmptyableServiceInformer {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		logging.FromContext(ctx).Panic(
			"Unable to fetch knative.dev/eventing/pkg/serving/EmptyableServiceInformer from context.")
	}
	return untyped.(informerv1.EmptyableServiceInformer)
}

var (
	_ controller.Informer                 = (*emptyableImpl)(nil)
	_ informerv1.EmptyableServiceInformer = (*emptyableImpl)(nil)
)

type emptyableImpl struct {
	internal v1.ServiceInformer
}

func (i *emptyableImpl) IsEmpty() bool {
	return (i.internal == nil)
}

func (i *emptyableImpl) GetInternal() v1.ServiceInformer {
	return i.internal
}

func (i *emptyableImpl) Run(stopCh <-chan struct{}) {
	if i.internal != nil {
		i.internal.Informer().Run(stopCh)
	} else {
		<-stopCh
	}
}

func (i *emptyableImpl) HasSynced() bool {
	if i.internal != nil {
		return i.internal.Informer().HasSynced()
	}
	return true
}
