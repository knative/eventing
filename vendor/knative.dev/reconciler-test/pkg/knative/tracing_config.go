/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package knative

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	configtracing "knative.dev/pkg/tracing/config"

	"knative.dev/reconciler-test/pkg/environment"
)

type tracingConfigEnvKey struct{}

func WithTracingConfig(ctx context.Context, env environment.Environment) (context.Context, error) {
	knativeNamespace := KnativeNamespaceFromContext(ctx)
	cm, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(knativeNamespace).Get(context.Background(), configtracing.ConfigName, metav1.GetOptions{})
	if err != nil {
		return ctx, fmt.Errorf("error while retrieving the %s config map in namespace %s: %+v", configtracing.ConfigName, knativeNamespace, errors.WithStack(err))
	}

	config, err := configtracing.NewTracingConfigFromConfigMap(cm)
	if err != nil {
		return ctx, fmt.Errorf("error while parsing the %s config map in namespace %s: %+v", configtracing.ConfigName, knativeNamespace, errors.WithStack(err))
	}

	configSerialized, err := configtracing.TracingConfigToJSON(config)
	if err != nil {
		return ctx, fmt.Errorf("error while serializing the %s config map in namespace %s: %+v", configtracing.ConfigName, knativeNamespace, errors.WithStack(err))
	}

	return context.WithValue(ctx, tracingConfigEnvKey{}, configSerialized), nil
}

var _ environment.EnvOpts = WithTracingConfig

func TracingConfigFromContext(ctx context.Context) string {
	if e, ok := ctx.Value(tracingConfigEnvKey{}).(string); ok {
		return e
	}
	panic("no tracing config found in the context, make sure you properly configured the env opts using WithTracingConfig")
}
