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

package resource

import (
	"errors"
	"fmt"
	"os"

	"knative.dev/pkg/changeset"
	"knative.dev/pkg/system"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

const (
	otelServiceNameKey = "OTEL_SERVICE_NAME"
)

func Default(serviceName string) (*resource.Resource, error) {
	if name := os.Getenv(otelServiceNameKey); name != "" {
		serviceName = name
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceVersion(changeset.Get()),
		semconv.ServiceName(serviceName),
	}

	var err error

	if namespace := os.Getenv(system.NamespaceEnvKey); namespace != "" {
		attrs = append(attrs, semconv.K8SNamespaceName(namespace))
	} else {
		err = fmt.Errorf(
			"the environment variable %q is not set, not adding %q to otel attributes",
			system.NamespaceEnvKey,
			semconv.K8SNamespaceNameKey,
		)
	}

	resource, resourceErr := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			attrs...,
		),
	)

	if resourceErr != nil {
		err = errors.Join(err, fmt.Errorf("encountered error while merging otel resources: %s", resourceErr.Error()))
	}

	return resource, err
}
