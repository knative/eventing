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

	inmemorychannelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1alpha1/inmemorychannel"
	"knative.dev/pkg/logging"

	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	pkgreconciler "knative.dev/pkg/reconciler"
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/swappable"
	inmemorychannelinformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1alpha1/inmemorychannel"
	"knative.dev/eventing/pkg/inmemorychannel"
	"knative.dev/eventing/pkg/tracing"
)

const (
	readTimeout  = 15 * time.Minute
	writeTimeout = 15 * time.Minute
	port         = 8080
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Setup trace publishing.
	iw := cmw.(*configmap.InformedWatcher)
	if err := tracing.SetupDynamicPublishing(logger, iw, "imc-dispatcher", tracingconfig.ConfigName); err != nil {
		logger.Fatalw("Error setting up trace publishing", zap.Error(err))
	}

	sh, err := swappable.NewEmptyMessageHandler(ctx, logger.Desugar())
	if err != nil {
		logger.Fatalw("Error creating swappable.MessageHandler", zap.Error(err))
	}

	args := &inmemorychannel.InMemoryMessageDispatcherArgs{
		Port:         port,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      sh,
		Logger:       logger.Desugar(),
	}
	inMemoryDispatcher := inmemorychannel.NewMessageDispatcher(args)

	inmemorychannelInformer := inmemorychannelinformer.Get(ctx)
	informer := inmemorychannelInformer.Informer()

	r := &Reconciler{
		dispatcher:              inMemoryDispatcher,
		inmemorychannelLister:   inmemorychannelInformer.Lister(),
		inmemorychannelInformer: informer,
	}
	impl := inmemorychannelreconciler.NewImpl(ctx, r)

	// Nothing to filer, enqueue all imcs if configmap updates.
	noopFilter := func(interface{}) bool { return true }
	resyncIMCs := configmap.TypeFilter(channel.EventDispatcherConfig{})(func(string, interface{}) {
		impl.FilteredGlobalResync(noopFilter, informer)
	})
	// Watch for configmap changes and trigger imc reconciliation by enqueuing imcs.
	configStore := channel.NewEventDispatcherConfigStore(logging.FromContext(ctx), resyncIMCs)
	configStore.WatchConfigs(cmw)
	r.configStore = configStore

	logging.FromContext(ctx).Info("Setting up event handlers")

	// Watch for inmemory channels.
	r.inmemorychannelInformer.AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: filterWithAnnotation(injection.HasNamespaceScope(ctx)),
			Handler:    controller.HandleAll(impl.Enqueue),
		})

	// Start the dispatcher.
	go func() {
		err := inMemoryDispatcher.Start(ctx)
		if err != nil {
			logging.FromContext(ctx).Errorw("Failed stopping inMemoryDispatcher.", zap.Error(err))
		}
	}()

	return impl
}

func filterWithAnnotation(namespaced bool) func(obj interface{}) bool {
	if namespaced {
		return pkgreconciler.AnnotationFilterFunc(eventing.ScopeAnnotationKey, eventing.ScopeNamespace, false)
	}
	return pkgreconciler.AnnotationFilterFunc(eventing.ScopeAnnotationKey, eventing.ScopeCluster, true)
}
