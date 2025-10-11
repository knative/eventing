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
	"knative.dev/pkg/system"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/kelseyhightower/envconfig"

	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/observability/otel"

	"knative.dev/pkg/logging"

	"go.uber.org/zap"
	filteredconfigmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	"knative.dev/pkg/configmap"
	configmapinformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	k8sruntime "knative.dev/pkg/observability/runtime/k8s"
	pkgreconciler "knative.dev/pkg/reconciler"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/feature"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	eventpolicyinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
	eventtypeinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta3/eventtype"
	inmemorychannelinformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/inmemorychannel"
	inmemorychannelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/inmemorychannel"
	"knative.dev/eventing/pkg/inmemorychannel"
	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
)

const (
	readTimeout   = 15 * time.Minute
	writeTimeout  = 15 * time.Minute
	httpPort      = 8080
	httpsPort     = 8443
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

	pprof := k8sruntime.NewProfilingServer(logger.Named("pprof"))

	mp, tp := otel.SetupObservabilityOrDie(ctx, "inmemorychannel.dispatcher", logger, pprof)

	trustBundleConfigMapLister := filteredconfigmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector).Lister().ConfigMaps(system.Namespace())

	iw := cmw.(*configmapinformer.InformedWatcher)
	iw.Watch(o11yconfigmap.Name(), pprof.UpdateFromConfigMap)

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

	sh := multichannelfanout.NewEventHandler(ctx, logger.Desugar())

	inmemorychannelInformer := inmemorychannelinformer.Get(ctx)

	readinessChecker := &DispatcherReadyChecker{
		chLister:     inmemorychannelInformer.Lister(),
		chMsgHandler: sh,
	}

	oidcTokenProvider := auth.NewOIDCTokenProvider(ctx)

	clientConfig := eventingtls.ClientConfig{
		TrustBundleConfigMapLister: trustBundleConfigMapLister,
	}

	var globalResync func(obj interface{})

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(_ string, _ interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}
	})

	featureStore.WatchConfigs(cmw)
	r := &Reconciler{
		multiChannelEventHandler: sh,
		messagingClientSet:       eventingclient.Get(ctx).MessagingV1(),
		eventingClient:           eventingclient.Get(ctx).EventingV1beta3(),
		eventTypeLister:          eventtypeinformer.Get(ctx).Lister(),
		eventDispatcher: kncloudevents.NewDispatcher(
			clientConfig,
			oidcTokenProvider,
			kncloudevents.WithMeterProvider(mp),
			kncloudevents.WithTraceProvider(tp),
		),
		authVerifier:          auth.NewVerifier(ctx, eventpolicyinformer.Get(ctx).Lister(), trustBundleConfigMapLister, cmw),
		clientConfig:          clientConfig,
		inMemoryChannelLister: inmemorychannelInformer.Lister(),
		meterProvider:         mp,
		traceProvider:         tp,
	}

	impl := inmemorychannelreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{SkipStatusUpdates: true, FinalizerName: finalizerName, ConfigStore: featureStore}
	})
	// Create event recorder for emitting K8s events when messages are dropped.
	// This recorder will be passed to the MultiChannelEventHandler, which will
	// propagate it to all fanout handlers.
	eventRecorder := controller.GetEventRecorder(ctx)
	if eventRecorder != nil {
		logger.Info("Setting event recorder on MultiChannelEventHandler for dropped message notifications")
		sh.SetEventRecorder(eventRecorder)
	} else {
		logger.Warn("Event recorder not available, dropped messages will only be logged")
	}

	globalResync = func(_ interface{}) {
		impl.GlobalResync(inmemorychannelInformer.Informer())
	}

	r.featureStore = featureStore

	// Watch for inmemory channels.
	inmemorychannelInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: filterWithAnnotation(injection.HasNamespaceScope(ctx)),
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    impl.Enqueue,
				UpdateFunc: controller.PassNew(impl.Enqueue),
				DeleteFunc: r.deleteFunc,
			}})

	httpArgs := &inmemorychannel.InMemoryEventDispatcherArgs{
		Port:         httpPort,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      sh,
		Logger:       logger.Desugar(),

		HTTPEventReceiverOptions: []kncloudevents.HTTPEventReceiverOption{
			kncloudevents.WithChecker(readinessCheckerHTTPHandler(readinessChecker)),
		},
	}
	httpDispatcher := inmemorychannel.NewEventDispatcher(httpArgs)
	httpReceiver := httpDispatcher.GetReceiver()

	secret := types.NamespacedName{
		Namespace: system.Namespace(),
		Name:      eventingtls.IMCDispatcherServerTLSSecretName,
	}
	serverTLSConfig := eventingtls.NewDefaultServerConfig()
	serverTLSConfig.GetCertificate = eventingtls.GetCertificateFromSecret(ctx, secretinformer.Get(ctx), kubeclient.Get(ctx), secret)
	tlsConfig, err := eventingtls.GetTLSServerConfig(serverTLSConfig)
	if err != nil {
		logger.Panicf("unable to get tls config: %s", err)
	}
	httpsArgs := &inmemorychannel.InMemoryEventDispatcherArgs{
		Port:         httpsPort,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      sh,
		Logger:       logger.Desugar(),

		HTTPEventReceiverOptions: []kncloudevents.HTTPEventReceiverOption{kncloudevents.WithTLSConfig(tlsConfig)},
	}
	httpsDispatcher := inmemorychannel.NewEventDispatcher(httpsArgs)
	httpsReceiver := httpsDispatcher.GetReceiver()

	s, err := eventingtls.NewServerManager(ctx, &httpReceiver, &httpsReceiver, httpDispatcher.GetHandler(ctx), cmw)
	if err != nil {
		logger.Panicf("unable to initialize server manager: %s", err)
	}

	// Start the dispatcher.
	go func() {
		err := s.StartServers(ctx)

		if err != nil {
			logging.FromContext(ctx).Errorw("Failed stopping inMemoryDispatcher.", zap.Error(err))
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := mp.Shutdown(ctx); err != nil {
			logger.Errorw("Error flushing metrics", zap.Error(err))
		}

		if err := tp.Shutdown(ctx); err != nil {
			logger.Errorw("Error flushing traces", zap.Error(err))
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