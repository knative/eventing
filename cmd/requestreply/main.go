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

package main

import (
	"context"
	"log"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/requestreply"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	filteredfactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	configmap "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"

	requestreplyinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/requestreply"
)

type envConfig struct {
	HttpPort    int    `envconfig:"HTTP_PORT" default:"8080"`
	HttpsPort   int    `envconfig:"HTTPS_PORT" default:"8443"`
	PodIdx      int    `envconfig:"POD_INDEX"`
	SecretsPath string `envconfig:"SECRETS_PATH"`
}

func main() {
	ctx := signals.NewContext()

	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx = injection.WithConfig(ctx, cfg)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var:", err)
	}

	ctx = filteredfactory.WithSelectors(ctx,
		eventingtls.TrustBundleLabelSelector,
	)

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	loggingConfig, err := getLoggingConfig(ctx, system.Namespace(), logging.ConfigMapName())
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration:", err)
	}

	sl, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, "request-reply")
	defer flush(sl)

	ctx = logging.WithLogger(ctx, sl)

	logger := sl.Desugar()

	logger.Info("Starting the RequestReply Data Plane")

	kubeClient := kubeclient.Get(ctx)

	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())

	configMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(sl, atomicLevel, "request-reply"))

	trustBundleConfigMapLister := configmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector).Lister().ConfigMaps(system.Namespace())

	keyStore := &requestreply.AESKeyStore{
		Logger: sl.Named("key-store"),
	}

	err = keyStore.WatchPath(env.SecretsPath)
	if err != nil {
		logger.Fatal("failed to watch secrets file path", zap.Error(err))
	}
	defer keyStore.StopWatch()

	handler := requestreply.NewHandler(
		logger,
		requestreplyinformer.Get(ctx),
		trustBundleConfigMapLister,
		keyStore,
		env.PodIdx,
	)

	sm, err := eventingtls.NewServerManager(ctx,
		kncloudevents.NewHTTPEventReceiver(env.HttpPort),
		kncloudevents.NewHTTPEventReceiver(env.HttpsPort), // TODO: add tls config when we have it
		handler,
		configMapWatcher,
	)
	if err != nil {
		logger.Fatal("failed to start eventingtls server", zap.Error(err))
	}

	logger.Info("Starting informers")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatal("Failed to start informers", zap.Error(err))
	}

	logger.Info("Starting server")
	if err := sm.StartServers(ctx); err != nil {
		logger.Fatal("StartServers() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")
}

func flush(sl *zap.SugaredLogger) {
	_ = sl.Sync()
}

func getLoggingConfig(ctx context.Context, namespace, loggingConfigMapName string) (*logging.Config, error) {
	loggingConfigMap, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(namespace).Get(ctx, loggingConfigMapName, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		return logging.NewConfigFromConfigMap(nil)
	} else if err != nil {
		return nil, err
	}

	return logging.NewConfigFromConfigMap(loggingConfigMap)
}
