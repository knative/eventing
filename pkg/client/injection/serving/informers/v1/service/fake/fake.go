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

package fake

import (
	"context"

	informerv1 "knative.dev/eventing/pkg/client/injection/serving/informers/v1"
	service "knative.dev/eventing/pkg/client/injection/serving/informers/v1/service"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	v1 "knative.dev/serving/pkg/client/informers/externalversions/serving/v1"
	fake "knative.dev/serving/pkg/client/injection/informers/factory/fake"
)

var Get = service.Get

func init() {
	injection.Fake.RegisterInformer(withInformer)
}

func withInformer(ctx context.Context) (context.Context, controller.Informer) {
	f := fake.Get(ctx)
	var inf informerv1.EmptyableServiceInformer
	inf = &emptyableImpl{internal: f.Serving().V1().Services()}
	return context.WithValue(ctx, service.Key{}, inf), inf
}

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
