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

package dispatcher

import (
	"context"
	"time"

	"github.com/knative/eventing/pkg/inmemorychannel"
	"github.com/knative/eventing/pkg/provisioners/swappable"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"

	inmemorychannelinformer "github.com/knative/eventing/pkg/client/injection/informers/messaging/v1alpha1/inmemorychannel"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "InMemoryChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "in-memory-channel-dispatcher"

	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Minute
	port         = 8080
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	base := reconciler.NewBase(ctx, controllerAgentName, cmw)

	sh, err := swappable.NewEmptyHandler(base.Logger.Desugar())
	if err != nil {
		base.Logger.Fatal("Error creating swappable.Handler", zap.Error(err))
	}

	args := &inmemorychannel.InMemoryDispatcherArgs{
		Port:         port,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      sh,
		Logger:       base.Logger.Desugar(),
	}
	inMemoryDispatcher := inmemorychannel.NewDispatcher(args)

	inmemorychannelInformer := inmemorychannelinformer.Get(ctx)

	r := &Reconciler{
		Base:                    base,
		dispatcher:              inMemoryDispatcher,
		inmemorychannelLister:   inmemorychannelInformer.Lister(),
		inmemorychannelInformer: inmemorychannelInformer.Informer(),
	}
	r.impl = controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	// Watch for inmemory channels.
	r.inmemorychannelInformer.AddEventHandler(controller.HandleAll(r.impl.Enqueue))

	return r.impl
}
