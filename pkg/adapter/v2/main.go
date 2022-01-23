/*
Copyright 2021 The Knative Authors

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

package adapter

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	"knative.dev/eventing/pkg/metrics/source"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/tracing"

	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
)

const (
	defaultMetricsPort = 9092
)

// Adapter is the interface receive adapters are expected to implement
type Adapter interface {
	Start(ctx context.Context) error
}

type AdapterConstructor func(ctx context.Context, env EnvConfigAccessor, client cloudevents.Client) Adapter

// ControllerConstructor is the function signature for creating controllers synchronizing
// the multi-tenant receive adapter state
type ControllerConstructor func(ctx context.Context, adapter Adapter) *controller.Impl

type injectorEnabledKey struct{}

// WithInjectorEnabled signals to MainWithInjectors that it should try to run injectors.
// TODO: deprecated. Use WithController instead
func WithInjectorEnabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, injectorEnabledKey{}, struct{}{})
}

// IsInjectorEnabled checks the context for the desire to enable injectors
// TODO: deprecated.
func IsInjectorEnabled(ctx context.Context) bool {
	val := ctx.Value(injectorEnabledKey{})
	return val != nil
}

func Main(component string, ector EnvConfigConstructor, ctor AdapterConstructor) {
	ctx := signals.NewContext()
	MainWithContext(ctx, component, ector, ctor)
}

func MainWithContext(ctx context.Context, component string, ector EnvConfigConstructor, ctor AdapterConstructor) {
	MainWithEnv(ctx, component, ConstructEnvOrDie(ector), ctor)
}

func MainWithEnv(ctx context.Context, component string, env EnvConfigAccessor, ctor AdapterConstructor) {
	if flag.Lookup("disable-ha") == nil {
		flag.Bool("disable-ha", false, "Whether to disable high-availability functionality for this component.")
	}

	if ControllerFromContext(ctx) != nil || IsInjectorEnabled(ctx) {
		ictx, informers := SetupInformers(ctx, env.GetLogger())
		if informers != nil {
			StartInformers(ctx, informers) // none-blocking
		}
		ctx = ictx
	}

	if !flag.Parsed() {
		flag.Parse()
	}

	b, err := strconv.ParseBool(flag.Lookup("disable-ha").Value.String())
	if err != nil || b {
		ctx = withHADisabledFlag(ctx)
	}

	MainWithInformers(ctx, component, env, ctor)
}

func MainWithInformers(ctx context.Context, component string, env EnvConfigAccessor, ctor AdapterConstructor) {
	if !flag.Parsed() {
		flag.Parse()
	}
	env.SetComponent(component)

	// Configure a logger based on environment variables. If the
	// configuration is not present it will return a default logger.
	//
	// If the logger needs to be constructed based on a ConfigMap
	// it will be overwriten, but configuring this one early will
	// avoid errors from packages that expect the logger at context.
	logger := env.GetLogger()
	ctx = logging.WithLogger(ctx, logger)

	ctx = WithNamespace(ctx, env.GetNamespace())
	lcm := ConfigMapConfiguredLoggerFromContext(ctx)
	ocm := ConfigMapConfiguredObservabilityFromContext(ctx)
	tcm := ConfigMapConfiguredTracingFromContext(ctx)

	// If the adapter needs to watch ConfigMaps
	// configure the watcher and add it to the context.
	if lcm != "" || ocm != "" || tcm != "" {
		cmw := SetupConfigMapWatch(ctx)
		ctx = WithConfigWatcher(ctx, cmw)
	}

	if lcm != "" {
		var lc *logging.Config
		cm, err := GetConfigMapByPolling(ctx, lcm)
		if err != nil {
			logger.Warnw("logging ConfigMap "+lcm+" could not be retrieved, falling back to default", zap.Error(err))
			lc, err = logging.NewConfigFromMap(nil)
		} else {
			lc, err = logging.NewConfigFromConfigMap(cm)
		}

		if err != nil {
			logger.Fatal("could not build the logging configuration", zap.Error(err))
		}

		// Flush temporary initial logger and overrwrite with the
		// one created from the ConfigMap.
		_ = logger.Sync()
		logger, atomicLevel := SetupLogger(lc, component)
		ctx = logging.WithLogger(ctx, logger)

		cmw := ConfigWatcherFromContext(ctx)
		cmw.Watch(lcm, logging.UpdateLevelFromConfigMap(logger, atomicLevel, component))
		if err := cmw.Start(ctx.Done()); err != nil {
			logger.Fatalw("Failed to start configuration manager", zap.Error(err))
		}
	}

	// Now that the definitive logger has been configured
	// we can defer flushing it.
	defer flush(logger)

	// Report stats on Go memory usage every 30 seconds.
	metrics.MemStatsOrDie(ctx)

	var crStatusEventClient *crstatusevent.CRStatusEventClient

	// Setup metrics and tracing according to the ConfigMap data.
	if ocm != "" {
		// Make sure the ConfigMap exists
		cm, err := GetConfigMapByPolling(ctx, ocm)
		if err != nil {
			logger.Fatalw("error reading "+ocm+" ConfigMap", zap.Error(err))
		}

		profilingHandler := profiling.NewHandler(logger, false)
		profilingServer := profiling.NewServer(profilingHandler)

		go func() {
			// Don't forward ErrServerClosed as that indicates we're already shutting down.
			if err := profilingServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Errorw("Profiling server failed", zap.Error(err))
			}
		}()

		cmw := ConfigWatcherFromContext(ctx)

		// Metrics exporter is configured
		// through the observability ConfigMap.
		updateMetricsFunc, err := metrics.UpdateExporterFromConfigMapWithOpts(ctx, metrics.ExporterOptions{
			Component:      component,
			PrometheusPort: defaultMetricsPort,
			Secrets:        SecretFetcher(ctx),
		}, logger)
		if err != nil {
			logger.Fatal("Failed to create metrWics exporter update function", zap.Error(err))
		}

		// CloudEvents Client status reporter is configured
		// through the observability ConfigMap.
		crStatusEventClient = crstatusevent.NewCRStatusEventClient(cm.Data)

		cmw.Watch(ocm,
			updateMetricsFunc,
			profilingHandler.UpdateFromConfigMap,
			crstatusevent.UpdateFromConfigMap(crStatusEventClient))

	} else {
		// Convert json metrics.ExporterOptions to metrics.ExporterOptions.
		if metricsConfig, err := env.GetMetricsConfig(); err != nil {
			logger.Errorw("Failed to process metrics options", zap.Error(err))
		} else if metricsConfig != nil {
			if err := metrics.UpdateExporter(ctx, *metricsConfig, logger); err != nil {
				logger.Errorw("Failed to create the metrics exporter", zap.Error(err))
			}
			// Check if metrics config contains profiling flag
			if metricsConfig.ConfigMap != nil {
				if enabled, err := profiling.ReadProfilingFlag(metricsConfig.ConfigMap); err == nil && enabled {
					// Start a goroutine to server profiling metrics
					logger.Info("Profiling enabled")
					go func() {
						server := profiling.NewServer(profiling.NewHandler(logger, true))
						// Don't forward ErrServerClosed as that indicates we're already shutting down.
						if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
							logger.Errorw("Profiling server failed", zap.Error(err))
						}
					}()
				}
				crStatusEventClient = crstatusevent.NewCRStatusEventClient(metricsConfig.ConfigMap)

			}
		}
	}

	if tcm != "" {
		cmw := ConfigWatcherFromContext(ctx)
		bin := fmt.Sprintf("%s.%s", env.GetName(), NamespaceFromContext(ctx))

		if err := tracing.SetupDynamicPublishing(logger, cmw, bin, tcm); err != nil {
			logger.Errorw("Error setting up trace publishing. Tracing configuration will be ignored.", zap.Error(err))
		}
	} else {
		if err := env.SetupTracing(logger); err != nil {
			// If tracing doesn't work, we will log an error, but allow the adapter
			// to continue to start.
			logger.Errorw("Error setting up trace publishing", zap.Error(err))
		}
	}

	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Errorw("Error building statsreporter", zap.Error(err))
	}

	eventsClient, err := NewCloudEventsClientCRStatus(env, reporter, crStatusEventClient)
	if err != nil {
		logger.Fatalw("Error building cloud event client", zap.Error(err))
	}

	// Configuring the adapter
	adapter := ctor(ctx, env, eventsClient)

	// Build the leader elector
	leConfig, err := env.GetLeaderElectionConfig()
	if err != nil {
		logger.Errorw("Error loading the leader election configuration", zap.Error(err))
	}

	if !isHADisabledFlag(ctx) && IsHAEnabled(ctx) {
		// Signal that we are executing in a context with leader election.
		logger.Info("Leader election mode enabled")
		ctx = leaderelection.WithStandardLeaderElectorBuilder(ctx, kubeclient.Get(ctx), *leConfig)
	}

	wg := sync.WaitGroup{}

	// Create and start controller is needed
	if ctor := ControllerFromContext(ctx); ctor != nil {
		ctrl := ctor(ctx, adapter)

		if leaderelection.HasLeaderElection(ctx) {
			// the reconciler MUST implement LeaderAware.
			if _, ok := ctrl.Reconciler.(reconciler.LeaderAware); !ok {
				log.Fatalf("%T is not leader-aware, all reconcilers must be leader-aware to enable fine-grained leader election.", ctrl.Reconciler)
			}
		}

		logger.Info("Starting controller")
		wg.Add(1)
		go func() {
			defer wg.Done()
			controller.StartAll(ctx, ctrl)
		}()
	}

	// Finally start the adapter (blocking)
	if err := adapter.Start(ctx); err != nil {
		logger.Fatalw("Start returned an error", zap.Error(err))
	}

	wg.Wait()
}

func ConstructEnvOrDie(ector EnvConfigConstructor) EnvConfigAccessor {
	env := ector()
	if err := envconfig.Process("", env); err != nil {
		log.Panicf("Error processing env var: %s", err)
	}
	return env
}

func SetupInformers(ctx context.Context, logger *zap.SugaredLogger) (context.Context, []controller.Informer) {
	// Run the injectors, but only if strictly necessary to relax the dependency on kubeconfig.
	if len(injection.Default.GetInformers()) > 0 || len(injection.Default.GetClients()) > 0 ||
		len(injection.Default.GetDucks()) > 0 || len(injection.Default.GetInformerFactories()) > 0 {
		logger.Infof("Registering %d clients", len(injection.Default.GetClients()))
		logger.Infof("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
		logger.Infof("Registering %d informers", len(injection.Default.GetInformers()))
		logger.Infof("Registering %d ducks", len(injection.Default.GetDucks()))

		cfg := injection.ParseAndGetRESTConfigOrDie()
		return injection.Default.SetupInformers(ctx, cfg)
	}
	return ctx, nil
}

func StartInformers(ctx context.Context, informers []controller.Informer) {
	go func() {
		if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
			panic(fmt.Sprint("Failed to start informers - ", err))
		}
		<-ctx.Done()
	}()
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}

// GetConfigMapByPolling retrieves a ConfigMap.
// If an error other than NotFound is returned, the operation will be repeated
// each second up to 5 seconds.
// These timeout and retry interval are set by heuristics.
// e.g. istio sidecar needs a few seconds to configure the pod network.
//
// The context is expected to be initialized with injection and namespace.
func GetConfigMapByPolling(ctx context.Context, name string) (cm *corev1.ConfigMap, err error) {
	err = wait.PollImmediate(1*time.Second, 5*time.Second, func() (bool, error) {
		cm, err = kubeclient.Get(ctx).
			CoreV1().ConfigMaps(NamespaceFromContext(ctx)).
			Get(ctx, name, metav1.GetOptions{})
		return err == nil || apierrors.IsNotFound(err), nil
	})

	if err != nil {
		err = fmt.Errorf("timed out waiting for the condition: %w", err)
	}

	return cm, err
}

// SetupLogger sets up the logger using the config from the given context
// and returns a logger and atomic level, or dies by calling log.Fatalf.
func SetupLogger(config *logging.Config, component string) (*zap.SugaredLogger, zap.AtomicLevel) {
	l, level := logging.NewLoggerFromConfig(config, component)

	// If PodName is injected into the env vars, set it on the logger.
	if pn := os.Getenv("POD_NAME"); pn != "" {
		l = l.With(zap.String(logkey.Pod, pn))
	}

	return l, level
}

// ConfigMapWatchOptions are the options that can be set for
// the ConfigMapWatch informer.
type ConfigMapWatchOptions struct {
	LabelsFilter []labels.Requirement
}

// ConfigMapWatchOption modifies setup for a ConfigMap informer.
type ConfigMapWatchOption func(*ConfigMapWatchOptions)

// ConfigMapWatchWithLabels sets the labels filter to be
// configured at the ConfigMap watcher informer.
func ConfigMapWatchWithLabels(ls []labels.Requirement) ConfigMapWatchOption {
	return func(opts *ConfigMapWatchOptions) {
		opts.LabelsFilter = ls
	}
}

// SetupConfigMapWatch establishes a watch on a namespace's configmaps.
func SetupConfigMapWatch(ctx context.Context, opts ...ConfigMapWatchOption) *cminformer.InformedWatcher {
	o := &ConfigMapWatchOptions{}
	for _, opt := range opts {
		opt(o)
	}

	return cminformer.NewInformedWatcher(kubeclient.Get(ctx), NamespaceFromContext(ctx), o.LabelsFilter...)
}

// SecretFetcher provides a helper function to fetch individual Kubernetes
// Secrets (for example, a key for client-side TLS). Note that this is not
// intended for high-volume usage; the current use is when establishing a
// metrics client connection in WatchObservabilityConfigOrDie.
// This method requires that the Namespace has been added to the context.
func SecretFetcher(ctx context.Context) metrics.SecretFetcher {
	// NOTE: Do not use secrets.Get(ctx) here to get a SecretLister, as it will register
	// a *global* SecretInformer and require cluster-level `secrets.list` permission,
	// even if you scope down the Lister to a given namespace after requesting it. Instead,
	// we package up a function from kubeclient.
	// TODO(evankanderson): If this direct request to the apiserver on each TLS connection
	// to the opencensus agent is too much load, switch to a cached Secret.
	return func(name string) (*corev1.Secret, error) {
		return kubeclient.Get(ctx).CoreV1().Secrets(NamespaceFromContext(ctx)).Get(ctx, name, metav1.GetOptions{})
	}
}
