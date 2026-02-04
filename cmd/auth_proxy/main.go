/*
Copyright 2025 The Knative Authors

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

// Package main implements an authentication and authorization proxy that sits as a sidecar
// container alongside application pods. It performs OIDC-based authentication and policy-based
// authorization before forwarding requests to the target service. The proxy supports both HTTP
// and HTTPS traffic with configurable TLS settings.
package main

import (
	"context"
	"net"
	"os"
	"sync"

	//nolint:gosec
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	cmdbroker "knative.dev/eventing/cmd/broker"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/auth"
	eventingclient "knative.dev/eventing/pkg/client/clientset/versioned"
	eventinginformers "knative.dev/eventing/pkg/client/informers/externalversions"
	eventingv1alpha1listers "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	configmap "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
)

const component = "auth-proxy"

// envConfig holds all environment configuration for the auth proxy
type envConfig struct {
	TargetHost      string `envconfig:"TARGET_HOST" default:"localhost" required:"true"`
	TargetHTTPPort  int    `envconfig:"TARGET_HTTP_PORT"  default:"8080"`
	TargetHTTPSPort int    `envconfig:"TARGET_HTTPS_PORT"  default:"8443"`
	ProxyHTTPPort   int    `envconfig:"PROXY_HTTP_PORT" default:"3128"`
	ProxyHTTPSPort  int    `envconfig:"PROXY_HTTPS_PORT" default:"3129"`

	SinkURI       string  `envconfig:"SINK_URI" required:"true"`
	SinkNamespace string  `envconfig:"SINK_NAMESPACE" required:"true"`
	SinkAudience  *string `envconfig:"SINK_AUDIENCE"`

	// Parent resource information for dynamic EventPolicy lookup
	ParentAPIVersion string `envconfig:"PARENT_API_VERSION" required:"true"`
	ParentKind       string `envconfig:"PARENT_KIND" required:"true"`
	ParentName       string `envconfig:"PARENT_NAME" required:"true"`
	ParentNamespace  string `envconfig:"PARENT_NAMESPACE" required:"true"`

	SinkTLSCertPath *string `envconfig:"SINK_TLS_CERT_FILE"`
	SinkTLSKeyPath  *string `envconfig:"SINK_TLS_KEY_FILE"`
	SinkCACertsPath *string `envconfig:"SINK_TLS_CA_FILE"`
}

// ProxyHandler handles HTTP requests and performs authentication/authorization
// before forwarding to the target service
type ProxyHandler struct {
	kubeClient        kubernetes.Interface
	withContext       func(ctx context.Context) context.Context
	authVerifier      *auth.Verifier
	httpProxy         *httputil.ReverseProxy
	httpsProxy        *httputil.ReverseProxy
	config            envConfig
	eventPolicyLister eventingv1alpha1listers.EventPolicyLister
	parentResourceGVK schema.GroupVersionKind

	// Cache for subjects with filters
	subjectsWithFiltersMu     sync.RWMutex
	cachedSubjectsWithFilters []auth.SubjectsWithFilters
}

func main() {
	ctx := signals.NewContext()

	config, err := loadConfig()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	ctx, informers := setupInformers(ctx)
	configMapWatcher := configmap.NewInformedWatcher(kubeclient.Get(ctx), system.Namespace())
	logger := setupLogging(ctx, configMapWatcher)
	defer logger.Sync()
	ctx = logging.WithLogger(ctx, logger)

	featureStore := setupFeatureStore(ctx, logger, configMapWatcher)

	// Create namespace-scoped EventPolicy informer
	eventPolicyLister, eventPolicyInformer, err := setupEventPolicyInformer(ctx, config, logger)
	if err != nil {
		logger.Fatalw("Failed to setup EventPolicy informer", zap.Error(err))
	}
	informers = append(informers, eventPolicyInformer)

	handler, err := createProxyHandler(ctx, config, logger, featureStore, configMapWatcher, eventPolicyLister)
	if err != nil {
		logger.Fatalw("Failed to create proxy handler", zap.Error(err))
	}

	// Register event handler to invalidate cache when EventPolicies change
	registerEventPolicyHandler(eventPolicyInformer, handler, logger)

	serverManager, err := createServerManager(ctx, config, handler, logger, configMapWatcher)
	if err != nil {
		logger.Fatalw("Failed to create server manager", zap.Error(err))
	}

	if err := startServices(ctx, informers, configMapWatcher, logger); err != nil {
		logger.Fatalw("Failed to start services", zap.Error(err))
	}

	logger.Info("Starting auth proxy servers...")
	if err = serverManager.StartServers(ctx); err != nil {
		logger.Fatalw("StartServers() returned an error", zap.Error(err))
	}

	logger.Info("Exiting...")
}

// loadConfig loads and validates environment configuration
func loadConfig() (envConfig, error) {
	var config envConfig
	if err := envconfig.Process("", &config); err != nil {
		return config, fmt.Errorf("failed to process environment variables: %w", err)
	}
	return config, nil
}

// setupInformers initializes Kubernetes client and informers
func setupInformers(ctx context.Context) (context.Context, []controller.Informer) {
	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx = injection.WithConfig(ctx, cfg)
	ctx = filteredFactory.WithSelectors(ctx, eventingtls.TrustBundleLabelSelector)

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)
	return ctx, informers
}

// setupLogging initializes logging configuration and returns the logger
func setupLogging(ctx context.Context, cmw *configmap.InformedWatcher) *zap.SugaredLogger {
	loggingConfig, err := cmdbroker.GetLoggingConfig(ctx, system.Namespace(), logging.ConfigMapName())
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration:", err)
	}

	logger, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, component)

	cmw.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, component))

	return logger
}

// setupFeatureStore initializes feature flag store
func setupFeatureStore(_ context.Context, logger *zap.SugaredLogger, configMapWatcher *configmap.InformedWatcher) *feature.Store {
	featureStore := feature.NewStore(logger.Named("feature-config-store"))
	featureStore.WatchConfigs(configMapWatcher)
	return featureStore
}

// setupEventPolicyInformer creates a namespace-scoped EventPolicy informer
func setupEventPolicyInformer(ctx context.Context, config envConfig, logger *zap.SugaredLogger) (eventingv1alpha1listers.EventPolicyLister, cache.SharedIndexInformer, error) {
	cfg := injection.GetConfig(ctx)

	// Create eventing client
	eventingClient, err := eventingclient.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create eventing client: %w", err)
	}

	// Create namespace-scoped informer factory
	eventingInformerFactory := eventinginformers.NewSharedInformerFactoryWithOptions(
		eventingClient,
		controller.GetResyncPeriod(ctx),
		eventinginformers.WithNamespace(config.ParentNamespace),
	)

	eventPolicyInformer := eventingInformerFactory.Eventing().V1alpha1().EventPolicies()

	logger.Infof("Created namespace-scoped EventPolicy informer for namespace %s", config.ParentNamespace)

	return eventPolicyInformer.Lister(), eventPolicyInformer.Informer(), nil
}

// createProxyHandler creates and configures the proxy handler
func createProxyHandler(ctx context.Context, config envConfig, logger *zap.SugaredLogger, featureStore *feature.Store, configMapWatcher *configmap.InformedWatcher, eventPolicyLister eventingv1alpha1listers.EventPolicyLister) (*ProxyHandler, error) {
	// Parse parent resource GVK
	gv, err := schema.ParseGroupVersion(config.ParentAPIVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parent API version %q: %w", config.ParentAPIVersion, err)
	}
	parentGVK := gv.WithKind(config.ParentKind)

	handler := &ProxyHandler{
		kubeClient:        kubeclient.Get(ctx),
		authVerifier:      auth.NewVerifier(ctx, nil, nil, configMapWatcher),
		config:            config,
		eventPolicyLister: eventPolicyLister,
		parentResourceGVK: parentGVK,
	}

	handler.withContext = func(ctx context.Context) context.Context {
		return logging.WithLogger(featureStore.ToContext(ctx), logger)
	}

	httpProxy, err := httpReverseProxy(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP proxy: %w", err)
	}

	httpsProxy, err := httpsReverseProxy(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTPS proxy: %w", err)
	}

	handler.httpProxy = httpProxy
	handler.httpsProxy = httpsProxy

	return handler, nil
}

// createServerManager creates the TLS-enabled server manager
func createServerManager(ctx context.Context, config envConfig, handler *ProxyHandler, logger *zap.SugaredLogger, configMapWatcher *configmap.InformedWatcher) (*eventingtls.ServerManager, error) {
	var tlsConfig *tls.Config
	if handler.config.SinkTLSCertPath != nil && handler.config.SinkTLSKeyPath != nil {
		var err error
		tlsConfig, err = getServerTLSConfig(*handler.config.SinkTLSCertPath, *handler.config.SinkTLSKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS config: %w", err)
		}
		logger.Info("TLS config loaded successfully")
	}

	serverManager, err := eventingtls.NewServerManager(ctx,
		kncloudevents.NewHTTPEventReceiver(config.ProxyHTTPPort),
		kncloudevents.NewHTTPEventReceiver(config.ProxyHTTPSPort,
			kncloudevents.WithTLSConfig(tlsConfig)),
		handler,
		configMapWatcher,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create server manager: %w", err)
	}

	return serverManager, nil
}

// startServices starts all background services (configmap watcher and informers)
func startServices(ctx context.Context, informers []controller.Informer, configMapWatcher *configmap.InformedWatcher, logger *zap.SugaredLogger) error {
	logger.Debug("Starting ConfigMap watcher")
	if err := configMapWatcher.Start(ctx.Done()); err != nil {
		return fmt.Errorf("failed to start ConfigMap watcher: %w", err)
	}

	logger.Info("Starting informers")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		return fmt.Errorf("failed to start informers: %w", err)
	}

	return nil
}

// registerEventPolicyHandler registers an event handler on the EventPolicy informer
// to invalidate the cache when policies change
func registerEventPolicyHandler(eventPolicyInformer cache.SharedIndexInformer, handler *ProxyHandler, logger *zap.SugaredLogger) {
	_, _ = eventPolicyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			handler.invalidateSubjectsCache(logger)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			handler.invalidateSubjectsCache(logger)
		},
		DeleteFunc: func(obj interface{}) {
			handler.invalidateSubjectsCache(logger)
		},
	})
}

// invalidateSubjectsCache clears the cached subjects with filters
func (h *ProxyHandler) invalidateSubjectsCache(logger *zap.SugaredLogger) {
	h.subjectsWithFiltersMu.Lock()
	defer h.subjectsWithFiltersMu.Unlock()
	h.cachedSubjectsWithFilters = nil
	logger.Debug("Invalidated subjects cache due to EventPolicy change")
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r.Context())
	logger := logging.FromContext(ctx)
	features := feature.FromContext(ctx)

	logger.Debugf("Handling request to %s", r.RequestURI)

	// Get EventPolicies dynamically for the parent resource
	authSubjects, err := h.getAuthSubjects(logger)
	if err != nil {
		logger.Errorw("Failed to get EventPolicies", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = h.authVerifier.VerifyRequestFromSubjectsWithFilters(ctx, features, h.config.SinkAudience, authSubjects, h.config.SinkNamespace, r, w)
	if err != nil {
		logger.Debugw("Failed to verify AuthN and AuthZ", zap.Error(err))
		return
	}

	if r.TLS == nil {
		logger.Debug("Forwarding to HTTP target")
		h.httpProxy.ServeHTTP(w, r)
	} else {
		logger.Debug("Forwarding to HTTPS target")
		h.httpsProxy.ServeHTTP(w, r)
	}
}

// getAuthSubjects retrieves EventPolicies for the parent resource and converts them to SubjectsWithFilters
func (h *ProxyHandler) getAuthSubjects(logger *zap.SugaredLogger) ([]auth.SubjectsWithFilters, error) {
	// Try to use cached value (fast path with read lock)
	h.subjectsWithFiltersMu.RLock()
	if h.cachedSubjectsWithFilters != nil {
		cached := h.cachedSubjectsWithFilters
		h.subjectsWithFiltersMu.RUnlock()
		logger.Debug("Using cached subjects with filters")
		return cached, nil
	}
	h.subjectsWithFiltersMu.RUnlock()

	// Cache miss - acquire write lock and compute
	h.subjectsWithFiltersMu.Lock()
	defer h.subjectsWithFiltersMu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have populated the cache)
	if h.cachedSubjectsWithFilters != nil {
		logger.Debug("Using cached subjects with filters (after lock)")
		return h.cachedSubjectsWithFilters, nil
	}

	// Get EventPolicies applying to this resource
	policies, err := auth.GetEventPoliciesForResource(
		h.eventPolicyLister,
		h.parentResourceGVK,
		metav1.ObjectMeta{
			Name:      h.config.ParentName,
			Namespace: h.config.ParentNamespace,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get EventPolicies: %w", err)
	}

	logger.Debugf("Found %d EventPolicies for %s/%s", len(policies), h.config.ParentNamespace, h.config.ParentName)

	// Convert EventPolicies to SubjectsWithFilters
	subjectsWithFilters := make([]auth.SubjectsWithFilters, 0, len(policies))
	for _, policy := range policies {
		subjectsWithFilters = append(subjectsWithFilters, auth.SubjectsWithFilters{
			Subjects: policy.Status.From,
			Filters:  policy.Spec.Filters,
		})
	}

	// Store in cache
	h.cachedSubjectsWithFilters = subjectsWithFilters
	logger.Debugf("Cached %d subjects with filters for %s/%s", len(subjectsWithFilters), h.config.ParentNamespace, h.config.ParentName)

	return subjectsWithFilters, nil
}

// httpReverseProxy creates a reverse proxy for HTTP traffic to the target service
func httpReverseProxy(config envConfig) (*httputil.ReverseProxy, error) {
	httpTarget := fmt.Sprintf("http://%s:%d", config.TargetHost, config.TargetHTTPPort)

	httpTargetURL, err := url.Parse(httpTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to parse http target URL: %v", err)
	}

	return httputil.NewSingleHostReverseProxy(httpTargetURL), nil
}

// httpsReverseProxy creates a reverse proxy for HTTPS traffic with TLS configuration
func httpsReverseProxy(config envConfig) (*httputil.ReverseProxy, error) {
	sinkUrl, err := apis.ParseURL(config.SinkURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sink URL: %v", err)
	}

	httpsTarget := fmt.Sprintf("https://%s:%d", config.TargetHost, config.TargetHTTPSPort)

	httpsTargetURL, err := url.Parse(httpsTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to parse https target URL: %v", err)
	}

	httpsProxy := httputil.NewSingleHostReverseProxy(httpsTargetURL)
	httpsProxy.Director = func(req *http.Request) {
		// in case of https requests, we need to rewrite the request URL/host, as otherwise, we get a certificate validation error
		req.URL.Scheme = "https"
		req.URL.Host = httpsTargetURL.Host
		req.Host = sinkUrl.Host
	}

	var caCerts *string
	if config.SinkCACertsPath != nil {
		caCertsB, err := os.ReadFile(*config.SinkCACertsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificates from %s: %w", *config.SinkCACertsPath, err)
		}
		caCerts = ptr.To(string(caCertsB))
	}

	var base = http.DefaultTransport.(*http.Transport).Clone()
	clientConfig := eventingtls.ClientConfig{
		CACerts:                    caCerts,
		TrustBundleConfigMapLister: nil,
	}

	base.DialTLSContext = func(ctx context.Context, net, addr string) (net.Conn, error) {
		tlsConfig, err := eventingtls.GetTLSClientConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		tlsConfig.ServerName = sinkUrl.Host

		return network.DialTLSWithBackOff(ctx, net, fmt.Sprintf("%s:%d", config.TargetHost, config.TargetHTTPSPort), tlsConfig)
	}
	httpsProxy.Transport = base

	return httpsProxy, nil
}

// getServerTLSConfig creates TLS configuration for the server using certificate files
func getServerTLSConfig(serverTLSCertificatePath, serverTLSCertificateKeyPath string) (*tls.Config, error) {
	serverTLSConfig := eventingtls.NewDefaultServerConfig()
	serverTLSConfig.GetCertificate = func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		cert, err := tls.LoadX509KeyPair(serverTLSCertificatePath, serverTLSCertificateKeyPath)
		if err != nil {
			return nil, err
		}

		return &cert, nil
	}
	return eventingtls.GetTLSServerConfig(serverTLSConfig)
}
