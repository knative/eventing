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

	"knative.dev/pkg/injection"

	"k8s.io/client-go/tools/cache"

	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"knative.dev/pkg/kmeta"

	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/kncloudevents"

	"knative.dev/pkg/logging"

	"go.uber.org/zap"
	"knative.dev/pkg/configmap"
	configmapinformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	pkgreconciler "knative.dev/pkg/reconciler"

	"knative.dev/pkg/tracing"
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/channel"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	inmemorychannelinformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/inmemorychannel"
	inmemorychannelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/inmemorychannel"
	"knative.dev/eventing/pkg/inmemorychannel"
)

const (
	readTimeout   = 15 * time.Minute
	writeTimeout  = 15 * time.Minute
	port          = 8080
	finalizerName = "imc-dispatcher"
)

type envConfig struct {
	// TODO: change this environment variable to something like "PodGroupName".
	PodName       string `envconfig:"POD_NAME" required:"true"`
	ContainerName string `envconfig:"CONTAINER_NAME" required:"true"`

	// HTTP client conf used when dispatching events
	MaxIdleConns int `envconfig:"MAX_IDLE_CONNS" required:"true"`
	// MaxIdleConnsPerHost refers to the max idle connections per host, as in net/http/transport.
	MaxIdleConnsPerHost int `envconfig:"MAX_IDLE_CONNS_PER_HOST" required:"true"`
}

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Setup trace publishing.
	iw := cmw.(*configmapinformer.InformedWatcher)
	if err := tracing.SetupDynamicPublishing(logger, iw, "imc-dispatcher", tracingconfig.ConfigName); err != nil {
		logger.Panicw("Error setting up trace publishing", zap.Error(err))
	}
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Panicw("Failed to process env var", zap.Error(err))
	}

	// Setup connection arguments
	if env.MaxIdleConns <= 0 {
		logger.Panicf("MAX_IDLE_CONNS = %d. It must be greater than 0", env.MaxIdleConns)
	}
	if env.MaxIdleConnsPerHost <= 0 {
		logger.Panicf("MAX_IDLE_CONNS_PER_HOST = %d. It must be greater than 0", env.MaxIdleConnsPerHost)
	}
	kncloudevents.ConfigureConnectionArgs(&kncloudevents.ConnectionArgs{
		MaxIdleConns:        env.MaxIdleConns,
		MaxIdleConnsPerHost: env.MaxIdleConnsPerHost,
	})

	reporter := channel.NewStatsReporter(env.ContainerName, kmeta.ChildName(env.PodName, uuid.New().String()))

	sh := multichannelfanout.NewMessageHandler(ctx, logger.Desugar(), channel.NewMessageDispatcher(logger.Desugar()), reporter)

	readinessChecker := &DispatcherReadyChecker{
		chLister:     inmemorychannelinformer.Get(ctx).Lister(),
		chMsgHandler: sh,
	}

	args := &inmemorychannel.InMemoryMessageDispatcherArgs{
		Port:         port,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      sh,
		Logger:       logger.Desugar(),

		HTTPMessageReceiverOptions: []kncloudevents.HTTPMessageReceiverOption{
			kncloudevents.WithChecker(readinessCheckerHTTPHandler(readinessChecker)),
		},
	}
	inMemoryDispatcher := inmemorychannel.NewMessageDispatcher(args)

	inmemorychannelInformer := inmemorychannelinformer.Get(ctx)

	r := &Reconciler{
		multiChannelMessageHandler: sh,
		reporter:                   reporter,
		messagingClientSet:         eventingclient.Get(ctx).MessagingV1(),
	}
	impl := inmemorychannelreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{SkipStatusUpdates: true, FinalizerName: finalizerName}
	})

	// Watch for inmemory channels.
	inmemorychannelInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: filterWithAnnotation(injection.HasNamespaceScope(ctx)),
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    impl.Enqueue,
				UpdateFunc: controller.PassNew(impl.Enqueue),
				DeleteFunc: r.deleteFunc,
			}})

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
