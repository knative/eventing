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
	"strconv"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/tracing"

	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/eventingtls"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/signals"

	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
	"knative.dev/eventing/pkg/metrics/source"
)

// Adapter is the interface receive adapters are expected to implement
type Adapter interface {
	Start(ctx context.Context) error
}

type AdapterConstructor func(ctx context.Context, env EnvConfigAccessor, client cloudevents.Client) Adapter

// ControllerConstructor is the function signature for creating controllers synchronizing
// the multi-tenant receive adapter state
type ControllerConstructor func(ctx context.Context, adapter Adapter) *controller.Impl

// LoggerConfigurator configures the logger for an adapter.
type LoggerConfigurator interface {
	CreateLogger(ctx context.Context) *zap.SugaredLogger
}

// MetricsExporterConfigurator configures the metrics exporter for an adapter.
type MetricsExporterConfigurator interface {
	SetupMetricsExporter(ctx context.Context)
}

// TracingConfiguration for adapters.
type TracingConfiguration struct {
	InstanceName string
}

// TracingConfigurator configures the tracing settings for an adapter.
type TracingConfigurator interface {
	SetupTracing(ctx context.Context, cfg *TracingConfiguration) tracing.Tracer
}

// ObservabilityConfigurator groups the observability related methods
// that configure an adapter.
type ObservabilityConfigurator interface {
	LoggerConfigurator
	MetricsExporterConfigurator
	TracingConfigurator
}

// ProfilerConfigurator configures the profiling settings for an adapter.
type ProfilerConfigurator interface {
	CreateProfilingServer(ctx context.Context) *http.Server
}

// CloudEventsStatusReporterConfigurator configures the CloudEvents client reporting
// settings for an adapter.
type CloudEventsStatusReporterConfigurator interface {
	CreateCloudEventsStatusReporter(ctx context.Context) *crstatusevent.CRStatusEventClient
}

// AdapterConfigurator exposes methods for configuring the adapter.
type AdapterConfigurator interface {
	ObservabilityConfigurator
	ProfilerConfigurator
	CloudEventsStatusReporterConfigurator
}

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

	// If not explicitly set, use the namespace from the environment variable.
	if NamespaceFromContext(ctx) == "" {
		ctx = WithNamespace(ctx, env.GetNamespace())
	}

	// If required a ConfigMap watcher is made available for configuration, either at this
	// shared main function or at the downstream adapter's code.
	if IsConfigWatcherEnabled(ctx) {
		if cmw := ConfigWatcherFromContext(ctx); cmw == nil {
			ctx = WithConfigWatcher(ctx, SetupConfigMapWatch(ctx))
		}
	}

	// The adapter configurator is used to setup and customize the adapter behavior
	configurator := newConfigurator(env, ConfiguratorOptionsFromContext(ctx)...)

	logger := configurator.CreateLogger(ctx)
	defer flush(logger)

	// Flush any previous initial logger and replace with the
	// one created from the configurator.
	prev := logging.FromContext(ctx)
	_ = prev.Sync()
	ctx = logging.WithLogger(ctx, logger)

	configurator.SetupMetricsExporter(ctx)

	// Report stats on Go memory usage.
	metrics.MemStatsOrDie(ctx)

	// Create a profiling server based on configuration.
	if ps := configurator.CreateProfilingServer(ctx); ps != nil {
		go func() {
			// Don't forward ErrServerClosed as that indicates we're already shutting down.
			if err := ps.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Errorw("profiling server failed", zap.Error(err))
			}
		}()
	}

	tracer := configurator.SetupTracing(ctx, &TracingConfiguration{InstanceName: env.GetName()})
	defer tracer.Shutdown(context.Background())

	crStatusEventClient := configurator.CreateCloudEventsStatusReporter(ctx)

	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Errorw("Error building statsreporter", zap.Error(err))
	}

	var trustBundleConfigMapLister corev1listers.ConfigMapNamespaceLister
	if IsConfigWatcherEnabled(ctx) {

		logger.Info("ConfigMap watcher is enabled")

		// Manually create a ConfigMap informer for the env.GetNamespace() namespace to have it
		// optionally created when needed.
		infFactory := informers.NewSharedInformerFactoryWithOptions(
			kubeclient.Get(ctx),
			controller.GetResyncPeriod(ctx),
			informers.WithNamespace(env.GetNamespace()),
			informers.WithTweakListOptions(func(options *metav1.ListOptions) {
				options.LabelSelector = eventingtls.TrustBundleLabelSelector
			}),
		)

		go func() {
			<-ctx.Done()
			infFactory.Shutdown()
		}()

		inf := infFactory.Core().V1().ConfigMaps()

		_ = inf.Informer() // Actually create informer

		trustBundleConfigMapLister = inf.Lister().ConfigMaps(env.GetNamespace())

		infFactory.Start(ctx.Done())
		_ = infFactory.WaitForCacheSync(ctx.Done())
	}

	clientConfig := ClientConfig{
		Env:                        env,
		Reporter:                   reporter,
		CrStatusEventClient:        crStatusEventClient,
		TokenProvider:              auth.NewOIDCTokenProvider(ctx),
		TrustBundleConfigMapLister: trustBundleConfigMapLister,
	}
	ctx = withClientConfig(ctx, clientConfig)

	eventsClient, err := NewClient(clientConfig)
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

	if cmw := ConfigWatcherFromContext(ctx); cmw != nil {
		if err := cmw.Start(ctx.Done()); err != nil {
			logger.Fatalw("Failed to start configuration manager", zap.Error(err))
		}
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
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		cm, err = kubeclient.Get(ctx).
			CoreV1().ConfigMaps(NamespaceFromContext(ctx)).
			Get(ctx, name, metav1.GetOptions{})
		return err == nil || apierrors.IsNotFound(err), nil
	})

	if err != nil {
		err = fmt.Errorf("timed out waiting trying to retrieve ConfigMap: %w", err)
	}

	return cm, err
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
func SetupConfigMapWatch(ctx context.Context, opts ...ConfigMapWatchOption) configmap.Watcher {
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

// adapterConfigurator hosts the range of configurators that
// will be used when setting up the adapter.
type adapterConfigurator struct {
	LoggerConfigurator
	MetricsExporterConfigurator
	TracingConfigurator
	ProfilerConfigurator
	CloudEventsStatusReporterConfigurator
}

// ConfiguratorOption enables customizing the adapter configuration.
type ConfiguratorOption func(*adapterConfigurator)

// WithLoggerConfigurator sets the adapter configurator with
// a custom logger option.
func WithLoggerConfigurator(c LoggerConfigurator) ConfiguratorOption {
	return func(acfg *adapterConfigurator) {
		acfg.LoggerConfigurator = c
	}
}

// WithMetricsExporterConfigurator sets the adapter configurator with
// a custom metrics exporter option.
func WithMetricsExporterConfigurator(c MetricsExporterConfigurator) ConfiguratorOption {
	return func(acfg *adapterConfigurator) {
		acfg.MetricsExporterConfigurator = c
	}
}

// WithTracingConfigurator sets the adapter configurator with
// a custom tracing option.
func WithTracingConfigurator(c TracingConfigurator) ConfiguratorOption {
	return func(acfg *adapterConfigurator) {
		acfg.TracingConfigurator = c
	}
}

// WithProfilerConfigurator sets the adapter configurator with
// a custom profiler option.
func WithProfilerConfigurator(c ProfilerConfigurator) ConfiguratorOption {
	return func(acfg *adapterConfigurator) {
		acfg.ProfilerConfigurator = c
	}
}

// WithCloudEventsStatusReporterConfigurator sets the adapter configurator with
// a CloudEvents status reporter option.
func WithCloudEventsStatusReporterConfigurator(c CloudEventsStatusReporterConfigurator) ConfiguratorOption {
	return func(acfg *adapterConfigurator) {
		acfg.CloudEventsStatusReporterConfigurator = c
	}
}

// newConfigurator creates an adapter configurator that defaults to environment variable based
// internal configurators, and can be overridden to use custom ones.
func newConfigurator(env EnvConfigAccessor, opts ...ConfiguratorOption) AdapterConfigurator {
	// default to environment variable based configurators
	acfg := &adapterConfigurator{
		LoggerConfigurator:                    NewLoggerConfiguratorFromEnvironment(env),
		MetricsExporterConfigurator:           NewMetricsExporterConfiguratorFromEnvironment(env),
		TracingConfigurator:                   NewTracingConfiguratorFromEnvironment(env),
		ProfilerConfigurator:                  NewProfilerConfiguratorFromEnvironment(env),
		CloudEventsStatusReporterConfigurator: NewCloudEventsStatusReporterConfiguratorFromEnvironment(env),
	}

	// override with user defined options
	for _, opt := range opts {
		opt(acfg)
	}

	return acfg
}

var _ AdapterConfigurator = (*adapterConfigurator)(nil)
