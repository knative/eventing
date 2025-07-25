/*
Copyright 2023 The Knative Authors

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

package ingress

import (
	"context"
	"crypto/tls"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/observability/otel"
	"knative.dev/pkg/configmap"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
)

func NewServerManager(
	ctx context.Context,
	logger *zap.Logger,
	cmw configmap.Watcher,
	httpPort, httpsPort int,
	meterProvider metric.MeterProvider,
	traceProvider trace.TracerProvider,
	handler *Handler) (*eventingtls.ServerManager, error) {
	tlsConfig, err := getServerTLSConfig(ctx)
	if err != nil {
		logger.Info("failed to get TLS server config", zap.Error(err))
	}

	httpReceiver := kncloudevents.NewHTTPEventReceiver(httpPort)
	httpsReceiver := kncloudevents.NewHTTPEventReceiver(httpsPort, kncloudevents.WithTLSConfig(tlsConfig))

	otelHandler := otel.NewHandler(handler, "broker.ingress", meterProvider, traceProvider)
	return eventingtls.NewServerManager(ctx, httpReceiver, httpsReceiver, otelHandler, cmw)
}

func getServerTLSConfig(ctx context.Context) (*tls.Config, error) {
	secret := types.NamespacedName{
		Namespace: "knative-eventing",
		Name:      eventingtls.BrokerIngressServerTLSSecretName,
	}

	serverTLSConfig := eventingtls.NewDefaultServerConfig()
	serverTLSConfig.GetCertificate = eventingtls.GetCertificateFromSecret(ctx, secretinformer.Get(ctx), kubeclient.Get(ctx), secret)
	return eventingtls.GetTLSServerConfig(serverTLSConfig)
}
