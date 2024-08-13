/*
Copyright 2020 The Knative Authors

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

package eventshub

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"

	"knative.dev/reconciler-test/pkg/environment"
	eventshubrbac "knative.dev/reconciler-test/pkg/eventshub/rbac"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/knativeservice"
	"knative.dev/reconciler-test/pkg/resources/secret"
	"knative.dev/reconciler-test/pkg/resources/service"
	"knative.dev/reconciler-test/pkg/resources/serviceaccount"

	authv1 "k8s.io/api/authentication/v1"
)

const (
	OIDCTokenExpiryMinutes = 10
)

//go:embed 102-service.yaml 103-pod.yaml
var servicePodTemplates embed.FS

//go:embed 104-forwarder.yaml
var forwarderTemplates embed.FS

//go:embed 105-certificate-service.yaml
var serviceTLSCertificate embed.FS

//go:embed 105-issuer-ca.yaml 105-certificate-ca.yaml 105-issuer-certificate.yaml
var caTLSCertificates embed.FS

const (
	caSecretName = "eventshub-ca"
)

// Install starts a new eventshub with the provided name
// Note: this function expects that the Environment is configured with the
// following options, otherwise it will panic:
//
//	ctx, env := global.Environment(
//	  knative.WithKnativeNamespace("knative-namespace"),
//	  knative.WithLoggingConfig,
//	  knative.WithTracingConfig,
//	  k8s.WithEventListener,
//	)
func Install(name string, options ...EventsHubOption) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		if err := registerImage(ctx); err != nil {
			t.Fatalf("Failed to install eventshub image: %v", err)
		}
		env := environment.FromContext(ctx)
		namespace := env.Namespace()
		log := logging.FromContext(ctx)

		// Compute the user provided envs
		envs := make(map[string]string)
		if err := compose(options...)(ctx, envs); err != nil {
			log.Fatalf("Error while computing environment variables for eventshub: %s", err)
		}

		// eventshub needs tracing and logging config
		envs[ConfigLoggingEnv] = knative.LoggingConfigFromContext(ctx)
		envs[ConfigTracingEnv] = knative.TracingConfigFromContext(ctx)

		// Register the event info store to assert later the events published by the eventshub
		eventListener := k8s.EventListenerFromContext(ctx)
		registerEventsHubStore(ctx, eventListener, name, namespace)

		isReceiver := strings.Contains(envs[EventGeneratorsEnv], "receiver")
		isEnforceTLS := strings.Contains(envs[EnforceTLS], "true")
		isOIDCEnabled := strings.Contains(envs[OIDCEnabledEnv], "true")

		var withForwarder bool
		// Allow forwarder only when eventshub is receiver.
		if isForwarder(ctx) && isReceiver {
			withForwarder = isForwarder(ctx)
		}

		serviceName := name
		// When forwarder is included we need to rename the eventshub service to
		// prevent conflict with the forwarder service.
		if withForwarder {
			serviceName = feature.MakeRandomK8sName(name)
		}

		cfg := map[string]interface{}{
			"name":           name,
			"serviceName":    serviceName,
			"envs":           envs,
			"image":          ImageFromContext(ctx),
			"isReceiver":     isReceiver,
			"withOIDCAuth":   isOIDCEnabled,
			"withEnforceTLS": isEnforceTLS,
			"clusterDomain":  network.GetClusterDomainName(),
		}

		for k, v := range envs {
			if strings.HasPrefix(k, "TLS_ISSUER") {
				cfg[k] = v
			}
		}

		if isOIDCEnabled && !isReceiver {
			// install oidc sa
			oidcSAName := fmt.Sprintf("oidc-%s", name)
			if envs[OIDCSubjectEnv] != "" {
				oidcSAName = envs[OIDCSubjectEnv]
			}
			serviceaccount.Install(oidcSAName)(ctx, t)

			// generate token
			token := envs[OIDCTokenEnv]
			if token == "" {
				var err error

				token, err = generateOIDCToken(ctx, oidcSAName, envs)
				if err != nil {
					t.Fatalf("could not generate OIDC token: %w", err)
				}
			}

			// install oidc secret & enable mounting
			oidcSecretName := fmt.Sprintf("oidc-token-%s", name)
			secret.Install(oidcSecretName, secret.WithStringData(map[string]string{
				"token": token,
			}))(ctx, t)
			cfg["oidcSAName"] = oidcSAName

			// remove token from envs
			delete(envs, OIDCTokenEnv)
		}

		// Install ServiceAccount, Role, RoleBinding
		eventshubrbac.Install(cfg)(ctx, t)

		if ic := environment.GetIstioConfig(ctx); ic.Enabled {
			manifest.WithIstioPodAnnotations(cfg)
		}

		manifest.PodSecurityCfgFn(ctx, t)(cfg)

		if isEnforceTLS && isReceiver {
			if _, err := manifest.InstallYamlFS(ctx, serviceTLSCertificate, cfg); err != nil {
				log.Fatal(err)
			}
		}

		// Deploy Service/Pod
		if _, err := manifest.InstallYamlFS(ctx, servicePodTemplates, cfg); err != nil {
			log.Fatal(err)
		}

		k8s.WaitForPodReadyOrSucceededOrFail(ctx, t, name)

		// If the eventhubs starts an event receiver, we need to wait for the service endpoint to be synced
		if isReceiver {
			if isEnforceTLS {
				secret.IsPresent(fmt.Sprintf("server-tls-%s", name))(ctx, t)
			}

			k8s.WaitForServiceEndpointsOrFail(ctx, t, serviceName, 1)
			k8s.WaitForServiceReadyOrFail(ctx, t, serviceName, "/health/ready")
		}

		if withForwarder {
			sinkURL, err := service.Address(ctx, serviceName)
			if err != nil {
				log.Fatal(err)
			}
			// At this point env contains "receiver" so we need to override it.
			envs[EventGeneratorsEnv] = "forwarder"
			// No event recording desired, just logging.
			envs[EventLogsEnv] = "logger"
			cfg["envs"] = envs
			cfg["sink"] = sinkURL.URL.String()

			// Deploy Forwarder
			if _, err := manifest.InstallYamlFS(ctx, forwarderTemplates, cfg); err != nil {
				log.Fatal(err)
			}
			knativeservice.IsReady(name)
		}
	}
}

func WithTLS(t feature.T) environment.EnvOpts {
	return func(ctx context.Context, env environment.Environment) (context.Context, error) {

		return environment.WithPostInit(ctx, func(ctx context.Context, env environment.Environment) (context.Context, error) {

			if _, err := manifest.InstallYamlFS(ctx, caTLSCertificates, nil); err != nil {
				return ctx, fmt.Errorf("failed to install CA certificates and issuer: %w", err)
			}

			secret.IsPresent(caSecretName, secret.AssertKey("ca.crt"))(ctx, t)

			caCerts, err := getCACertsFromSecret(ctx)
			if err != nil {
				return ctx, err
			}
			s := string(caCerts)
			ctx = WithCaCerts(ctx, &s)

			return ctx, nil
		}), nil
	}
}

func ReceiverGVR(ctx context.Context) schema.GroupVersionResource {
	apiVersion := "v1"
	if isForwarder(ctx) {
		apiVersion = "serving.knative.dev/v1"
	}
	or := &corev1.ObjectReference{
		Kind:       "Service",
		Namespace:  environment.FromContext(ctx).Namespace(),
		APIVersion: apiVersion,
	}

	gvr, _ := meta.UnsafeGuessKindToResource(or.GroupVersionKind())
	return gvr
}

type caCertsKey struct{}

func WithCaCerts(ctx context.Context, caCerts *string) context.Context {
	return context.WithValue(ctx, caCertsKey{}, caCerts)
}

func GetCaCerts(ctx context.Context) *string {
	caCerts := ctx.Value(caCertsKey{})
	if caCerts == nil {
		return nil
	}
	return caCerts.(*string)
}

func getCACertsFromSecret(ctx context.Context) ([]byte, error) {
	ns := environment.FromContext(ctx).Namespace()

	s, err := kubeclient.Get(ctx).
		CoreV1().
		Secrets(ns).
		Get(ctx, caSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get eventshub CA certs secret: %w", err)
	}

	caCrt, _ := s.Data["ca.crt"]
	return caCrt, nil
}

func generateOIDCToken(ctx context.Context, saName string, envs map[string]string) (string, error) {
	kubeClient := kubeclient.Get(ctx)
	namespace := environment.FromContext(ctx).Namespace()

	wantExpiredToken := strings.Contains(envs[OIDCGenerateExpiredTokenEnv], "true")
	wantInvalidAudienceToken := strings.Contains(envs[OIDCGenerateInvalidAudienceTokenEnv], "true")
	wantCorruptedSignatureToken := strings.Contains(envs[OIDCGenerateCorruptedSignatureTokenEnv], "true")

	// we start with a valid token
	tokenRequest := authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences: []string{
				envs[OIDCSinkAudienceEnv],
			},
		},
	}

	if wantExpiredToken {
		tokenRequest.Spec.ExpirationSeconds = pointer.Int64(OIDCTokenExpiryMinutes * 60)
	}

	if wantInvalidAudienceToken {
		tokenRequest.Spec.Audiences = []string{
			fmt.Sprintf("invalid-%s", envs[OIDCSinkAudienceEnv]),
		}
	}

	// request token
	tokenRequestResponse, err := kubeClient.
		CoreV1().
		ServiceAccounts(namespace).
		CreateToken(context.TODO(), saName, &tokenRequest, metav1.CreateOptions{})

	if err != nil {
		return "", fmt.Errorf("could not request a token for %s: %w", saName, err)
	}

	// do post request steps
	token := tokenRequestResponse.Status.Token

	if wantCorruptedSignatureToken {
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			return "", fmt.Errorf("could not split token into header, payload and signature")
		}

		headerB64, payloadB64, signatureB64 := parts[0], parts[1], parts[2]

		signature, err := base64.RawURLEncoding.DecodeString(signatureB64)
		if err != nil {
			return "", fmt.Errorf("could not decode signature: %w", err)
		}
		signatureB64 = base64.RawURLEncoding.EncodeToString([]byte(signature[1:])) // remove first char from signature --> invalidate

		token = fmt.Sprintf("%s.%s.%s", headerB64, payloadB64, signatureB64)
	}

	return token, nil
}
