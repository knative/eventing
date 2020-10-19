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

package tracing

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"knative.dev/pkg/system"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	tracingconfig "knative.dev/pkg/tracing/config"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/test/zipkin"
)

// Setup sets up port forwarding to Zipkin.
func Setup(t *testing.T, client *testlib.Client) {
	zipkin.SetupZipkinTracingFromConfigTracingOrFail(context.Background(), t, client.Kube, system.Namespace())
}

// GetClusterDomain gets the Cluster's domain, e.g. 'cluster.local'.
func GetClusterDomain(t *testing.T, client *testlib.Client) string {
	d, err := getClusterDomain(context.Background(), client.Kube, system.Namespace())
	if err != nil {
		t.Fatal("Unable to get cluster domain:", err)
	}
	return d
}

func getClusterDomain(ctx context.Context, kubeClientset kubernetes.Interface, configMapNamespace string) (string, error) {
	// This a lightly altered version of knative.dev/pkg/test/zipkin/util.go's
	// SetupZipkinTracingFromConfigTracing.
	// TODO Move this function to knative/pkg.
	cm, err := kubeClientset.CoreV1().ConfigMaps(configMapNamespace).Get(ctx, "config-tracing", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error while retrieving config-tracing config map: %w", err)
	}
	c, err := tracingconfig.NewTracingConfigFromConfigMap(cm)
	if err != nil {
		return "", fmt.Errorf("error while parsing config-tracing config map: %w", err)
	}
	zipkinEndpointURL, err := url.Parse(c.ZipkinEndpoint)
	if err != nil {
		return "", fmt.Errorf("error while parsing the zipkin endpoint in config-tracing config map: %w", err)
	}
	domain, err := parseClusterDomainFromHostname(zipkinEndpointURL.Host)
	if err != nil {
		return "", fmt.Errorf("error while parsing the Zipkin endpoint in config-tracing config map: %w", err)
	}

	return domain, nil
}

func parseClusterDomainFromHostname(hostname string) (string, error) {
	// hostname will be something like 'name.ns.svc.clusterDomain:port' where clusterDomain is something
	// like 'cluster.local'.
	hostAndPort := strings.SplitN(hostname, ":", 2)
	host := hostAndPort[0]
	parts := strings.SplitN(host, ".", 4)
	if len(parts) < 3 || parts[2] != "svc" {
		return "", fmt.Errorf("could not extract namespace/name from %s", hostname)
	}
	if len(parts) == 3 {
		return "", nil
	}
	return parts[3], nil
}
