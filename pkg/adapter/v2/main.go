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
	"errors"
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

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
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
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
	"knative.dev/eventing/pkg/metrics/source"
)

const (
	defaultMetricsPort   = 9092
	defaultMetricsDomain = "knative.dev/eventing"
)

// Adapter is the interface receive adapters are expected to implement
type Adapter interface {
	Start(ctx context.Context) error
}

type AdapterConstructor func(ctx context.Context, env EnvConfigAccessor, client cloudevents.Client) Adapter

// ControllerConstructor is the function signature for creating controllers synchronizing
// the multi-tenant receive adapter state
type ControllerConstructor func(ctx context.Context, adapter Adapter) *controller.Impl

type AdapterDynamicConfigOption func(*AdapterDynamicConfig)

// metricDomainDefaulter is an AdapterDynamicconfig option that
// retrieves the adater's metric domain by looking at the
// environments metrics.DomainEnv variable first,
// then to the component's hardcoded default metrics domain,
// and if not present to the eventing default metrics domain.
func metricDomainDefaulter(adc *AdapterDynamicConfig) {
	if domain := os.Getenv(metrics.DomainEnv); domain != "" {
		adc.metricsDomain = domain
	} else if adc.metricsDomain == "" {
		adc.metricsDomain = defaultMetricsDomain
	}
}

// WithLoggerConfigMapName overrides the default logger ConfigMap name.
func WithLoggerConfigMapName(name string) AdapterDynamicConfigOption {
	return func(adc *AdapterDynamicConfig) {
		adc.loggingConfigName = name
	}
}

// WithLoggerConfigMapName overrides the default observability ConfigMap name.
func WithObservabilityConfigMapName(name string) AdapterDynamicConfigOption {
	return func(adc *AdapterDynamicConfig) {
		adc.observabilityConfigName = name
	}
}

// WithLoggerConfigMapName overrides the default tracing ConfigMap name.
func WithTracingConfigMapName(name string) AdapterDynamicConfigOption {
	return func(adc *AdapterDynamicConfig) {
		adc.tracingConfigName = name
	}
}

// NewAdapterDynamicConfig creates a dynamic configurator for adapters.
func NewAdapterDynamicConfig(opts ...AdapterDynamicConfigOption) *AdapterDynamicConfig {
	adc := &AdapterDynamicConfig{
		loggingConfigName:       logging.ConfigMapName(),
		observabilityConfigName: metrics.ConfigMapName(),
		tracingConfigName:       tracingconfig.ConfigName,
	}

	opts = append(opts, metricDomainDefaulter)

	for _, opt := range opts {
		opt(adc)
	}

	return adc
}

// AdapterDynamicConfig keeps the ConfigMap based dynamic
// configuration parameters for the adapter.
type AdapterDynamicConfig struct {
	// ConfigMap names
	loggingConfigName       string
	observabilityConfigName string
	tracingConfigName       string

	// Metrics domain
	metricsDomain string
}

// AdapterConfigurator exposes methods for configuring
// the adapter.
type AdapterConfigurator interface {
	CreateLogger(ctx context.Context) *zap.SugaredLogger
	SetupMetricsExporter(ctx context.Context)
	CreateProfilingServer(ctx context.Context) *http.Server
	CreateCloudEventsEventStatusReporter(ctx context.Context) *crstatusevent.CRStatusEventClient
	SetupTracing(ctx context.Context, instanceName string)
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

	var configurator AdapterConfigurator
	if ac := AdapterDynamicConfigFromContext(ctx); ac != nil {
		configurator = NewAdapterConfiguratorFromConfigMaps(ctx, component, ac)
	} else {
		configurator = NewAdapterConfiguratorFromEnvironment(env, component)
	}

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

	configurator.SetupTracing(ctx, env.GetName())

	crStatusEventClient := configurator.CreateCloudEventsEventStatusReporter(ctx)

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
		err = fmt.Errorf("timed out waiting trying to retrieve ConfigMap: %w", err)
	}

	return cm, err
}

// SetupLoggerFromConfig sets up the logger using the provided config
// and returns a logger and atomic level, or dies by calling log.Fatalf.
func SetupLoggerFromConfig(config *logging.Config, component string) (*zap.SugaredLogger, zap.AtomicLevel) {
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

type adapterConfiguratorEnvironment struct {
	env       EnvConfigAccessor
	component string
}

// NewAdapterConfiguratorFromEnvironment creates an adapter configurator based on
// environment variables from the accessor.
func NewAdapterConfiguratorFromEnvironment(env EnvConfigAccessor, component string) *adapterConfiguratorEnvironment {
	return &adapterConfiguratorEnvironment{
		env:       env,
		component: component,
	}
}

func (c *adapterConfiguratorEnvironment) CreateLogger(ctx context.Context) *zap.SugaredLogger {
	return c.env.GetLogger()
}

func (c *adapterConfiguratorEnvironment) getMetricsConfig() (*metrics.ExporterOptions, error) {
	metricsConfig, err := c.env.GetMetricsConfig()
	switch {
	case err != nil:
		return nil, fmt.Errorf("failed to process metrics options from environment: %w", err)
	case metricsConfig == nil || metricsConfig.ConfigMap == nil:
		return nil, errors.New("environment metrics options not provided")
	}
	return metricsConfig, nil
}

func (c *adapterConfiguratorEnvironment) SetupMetricsExporter(ctx context.Context) {
	logger := logging.FromContext(ctx)
	mc, err := c.getMetricsConfig()
	if err != nil {
		logger.Warn("metrics exporter not configured", zap.Error(err))
		return
	}

	if err = metrics.UpdateExporter(ctx, *mc, logger); err != nil {
		logger.Errorw("failed to create the metrics exporter", zap.Error(err))
	}
}

func (c *adapterConfiguratorEnvironment) CreateProfilingServer(ctx context.Context) *http.Server {
	logger := logging.FromContext(ctx)
	mc, err := c.getMetricsConfig()
	if err != nil {
		logger.Warn("profiler not configured", zap.Error(err))
		return nil
	}

	// Configure profiler using environment varibles.
	enabled, err := profiling.ReadProfilingFlag(mc.ConfigMap)
	switch {
	case err != nil:
		logger.Errorw("wrong profiler configuration", zap.Error(err))
		return nil
	case !enabled:
		return nil
	}

	return profiling.NewServer(profiling.NewHandler(logger, true))
}

func (c *adapterConfiguratorEnvironment) CreateCloudEventsEventStatusReporter(ctx context.Context) *crstatusevent.CRStatusEventClient {
	logger := logging.FromContext(ctx)
	mc, err := c.getMetricsConfig()
	if err != nil {
		logger.Warn("CloudEvents client reporter not configured", zap.Error(err))
		return nil
	}

	return crstatusevent.NewCRStatusEventClient(mc.ConfigMap)
}

func (c *adapterConfiguratorEnvironment) SetupTracing(ctx context.Context, instanceName string) {
	logger := logging.FromContext(ctx)
	if err := c.env.SetupTracing(logger); err != nil {
		// If tracing doesn't work, we will log an error, but allow the adapter
		// to continue to start.
		logger.Errorw("Error setting up trace publishing", zap.Error(err))
	}
}

type adapterConfiguratorConfigMap struct {
	adc       *AdapterDynamicConfig
	component string

	lcm *corev1.ConfigMap
	ocm *corev1.ConfigMap
}

// NewAdapterConfiguratorFromConfigMaps creates an adapter configurator based on ConfigMaps.
func NewAdapterConfiguratorFromConfigMaps(ctx context.Context, component string, adc *AdapterDynamicConfig) *adapterConfiguratorConfigMap {
	logger := logging.FromContext(ctx)

	// Create the Config Watcher if it has not been created yet.
	if cmw := ConfigWatcherFromContext(ctx); cmw == nil {
		logger.Info("Setting up ConfigMap Watcher")
		cmw := SetupConfigMapWatch(ctx)
		ctx = WithConfigWatcher(ctx, cmw)
	}

	// Retrieve current ConfigMaps
	lcm, err := GetConfigMapByPolling(ctx, adc.loggingConfigName)
	if err != nil {
		logger.Errorw("logging ConfigMap "+adc.loggingConfigName+" could not be retrieved", zap.Error(err))
	}
	ocm, err := GetConfigMapByPolling(ctx, adc.observabilityConfigName)
	if err != nil {
		logger.Errorw("observability ConfigMap "+adc.observabilityConfigName+" could not be retrieved", zap.Error(err))
	}

	return &adapterConfiguratorConfigMap{
		adc:       adc,
		component: component,
		lcm:       lcm,
		ocm:       ocm,
	}
}

func (c *adapterConfiguratorConfigMap) CreateLogger(ctx context.Context) *zap.SugaredLogger {
	// Use any pre-existing logger to inform
	logger := logging.FromContext(ctx)

	// Get logging configuration from ConfigMap or a
	// default configuration if the ConfigMap was not found.
	var lc *logging.Config
	var err error
	if c.lcm == nil {
		logger.Warn("logging configuration not found, falling back to defaults")
		lc, err = logging.NewConfigFromMap(nil)
	} else {
		lc, err = logging.NewConfigFromConfigMap(c.lcm)
	}

	// Not being able to create the logging configuration is not expected, in
	// such case panic.
	if err != nil {
		logger.Fatal("could not build the logging configuration", zap.Error(err))
	}

	logger, atomicLevel := SetupLoggerFromConfig(lc, c.component)

	logger.Infof("Adding Watcher on ConfigMap %s for logs", c.adc.loggingConfigName)

	cmw := ConfigWatcherFromContext(ctx)
	cmw.Watch(c.adc.loggingConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, c.component))
	if err := cmw.Start(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start configuration manager", zap.Error(err))
	}

	return logger
}

func (c *adapterConfiguratorConfigMap) CreateProfilingServer(ctx context.Context) *http.Server {
	logger := logging.FromContext(ctx)

	// Configure watcher to update profiler from ConfigMap.
	var enabled bool
	var err error
	if c.ocm == nil {
		logger.Warn("profiler configuration not found, falling back to disabling profiler requests")
		enabled = false
	} else if enabled, err = profiling.ReadProfilingFlag(c.ocm.Data); err != nil {
		logger.Errorw("wrong profiler configuration")
	}

	// Setup profiler even if it is disabled at the handler. Users
	// might activate it through the ConfigMap.
	profilingHandler := profiling.NewHandler(logger, enabled)
	profilingServer := profiling.NewServer(profilingHandler)

	logger.Infof("Adding Watcher on ConfigMap %s for profiler", c.adc.observabilityConfigName)
	ConfigWatcherFromContext(ctx).Watch(c.adc.observabilityConfigName, profilingHandler.UpdateFromConfigMap)

	return profilingServer
}

func (c *adapterConfiguratorConfigMap) SetupMetricsExporter(ctx context.Context) {
	logger := logging.FromContext(ctx)

	// Configure watcher to update metrics from ConfigMap.
	updateMetricsFunc, err := metrics.UpdateExporterFromConfigMapWithOpts(ctx, metrics.ExporterOptions{
		Domain:         c.adc.metricsDomain,
		Component:      c.component,
		PrometheusPort: defaultMetricsPort,
		Secrets:        SecretFetcher(ctx),
	}, logger)
	if err != nil {
		logger.Fatal("Failed to create metrics exporter update function", zap.Error(err))
	}

	logger.Infof("Adding Watcher on ConfigMap %s for metrics", c.adc.observabilityConfigName)
	ConfigWatcherFromContext(ctx).Watch(c.adc.observabilityConfigName, updateMetricsFunc)
}

func (c *adapterConfiguratorConfigMap) CreateCloudEventsEventStatusReporter(ctx context.Context) *crstatusevent.CRStatusEventClient {
	logger := logging.FromContext(ctx)

	var crStatusConfig map[string]string
	if c.ocm != nil {
		crStatusConfig = c.ocm.Data
	}
	r := crstatusevent.NewCRStatusEventClient(crStatusConfig)

	logger.Infof("Adding Watcher on ConfigMap %s for CE client status reporter", c.adc.observabilityConfigName)
	ConfigWatcherFromContext(ctx).Watch(c.adc.observabilityConfigName, crstatusevent.UpdateFromConfigMap(r))

	return r
}

func (c *adapterConfiguratorConfigMap) SetupTracing(ctx context.Context, instanceName string) {
	logger := logging.FromContext(ctx)

	cmw := ConfigWatcherFromContext(ctx)
	service := fmt.Sprintf("%s.%s", instanceName, NamespaceFromContext(ctx))

	logger.Infof("Adding Watcher on ConfigMap %s for tracing", c.adc.tracingConfigName)
	if err := tracing.SetupDynamicPublishing(logger, cmw, service, c.adc.tracingConfigName); err != nil {
		logger.Errorw("Error setting up trace publishing. Tracing configuration will be ignored.", zap.Error(err))
	}
}
