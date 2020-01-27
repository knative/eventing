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
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	controller "knative.dev/pkg/controller"
	injection "knative.dev/pkg/injection"
	logging "knative.dev/pkg/logging"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	v1 "knative.dev/serving/pkg/client/informers/externalversions/serving/v1"
	factory "knative.dev/serving/pkg/client/injection/informers/factory"
)

// This package register a serving v1 service informer as an "emptyable" informer.
// An "emptyable" informer delegates a real informer if the serving v1 API exists;
// otherwise it does nothing.
// This allows informer injection without knowing if the API exists in advance.
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
			inf := &EmptyableServiceInformer{}
			return context.WithValue(ctx, Key{}, inf), inf
		}
		logging.FromContext(ctx).Panic("Failed to check API version", servingv1.SchemeGroupVersion)
	}

	f := factory.Get(ctx)
	inf := NewEmptyableServiceInformer(f.Serving().V1().Services())
	return context.WithValue(ctx, Key{}, inf), inf
}

// Get extracts the typed informer from the context.
func Get(ctx context.Context) *EmptyableServiceInformer {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		logging.FromContext(ctx).Panic(
			"Unable to fetch knative.dev/eventing/pkg/client/injection/serving/informers/v1/service.EmptyableServiceInformer from context.")
	}
	return untyped.(*EmptyableServiceInformer)
}

var _ controller.Informer = (*EmptyableServiceInformer)(nil)

// EmptyableServiceInformer is an "emptyable" version of v1.ServiceInformer.
type EmptyableServiceInformer struct {
	internal v1.ServiceInformer
}

// NewEmptyableServiceInformer returns a new EmptyableServiceInformer.
func NewEmptyableServiceInformer(internal v1.ServiceInformer) *EmptyableServiceInformer {
	return &EmptyableServiceInformer{internal: internal}
}

// IsEmpty returns whether the informer is empty.
func (i *EmptyableServiceInformer) IsEmpty() bool {
	return (i.internal == nil)
}

// GetInternal returns the internal v1.ServiceInformer if the informer is not empty.
func (i *EmptyableServiceInformer) GetInternal() v1.ServiceInformer {
	return i.internal
}

// Run delegates to v1.ServiceInformer if the informer is not empty.
// Otherwise it does nothing.
func (i *EmptyableServiceInformer) Run(stopCh <-chan struct{}) {
	if i.internal != nil {
		i.internal.Informer().Run(stopCh)
	} else {
		<-stopCh
	}
}

// HasSynced delegates to v1.ServiceInformer if the informer is not empty.
// Otherwise it always returns true.
func (i *EmptyableServiceInformer) HasSynced() bool {
	if i.internal != nil {
		return i.internal.Informer().HasSynced()
	}
	return true
}
