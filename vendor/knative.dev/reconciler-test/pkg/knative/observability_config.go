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
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	o11yconfigmap "knative.dev/pkg/observability/configmap"

	"knative.dev/reconciler-test/pkg/environment"
)

type observabilityCfgEnvKey struct{}

func WithObservabilityConfig(ctx context.Context, env environment.Environment) (context.Context, error) {
	knativeNamespace := KnativeNamespaceFromContext(ctx)
	cm, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(knativeNamespace).Get(context.Background(), o11yconfigmap.Name(), metav1.GetOptions{})
	if err != nil {
		return ctx, fmt.Errorf("error while retrieving the %s config map in namespace %s: %+v", o11yconfigmap.Name(), knativeNamespace, errors.WithStack(err))
	}

	config, err := o11yconfigmap.Parse(cm)
	if err != nil {
		return ctx, fmt.Errorf("error while parsing the %s config map in namespace %s: %+v", o11yconfigmap.Name(), knativeNamespace, errors.WithStack(err))
	}

	configSerialized, err := json.Marshal(config)
	if err != nil {
		return ctx, fmt.Errorf("error while serializing the %s config map in namespace %s: %+v", o11yconfigmap.Name(), knativeNamespace, errors.WithStack(err))
	}

	return context.WithValue(ctx, observabilityCfgEnvKey{}, string(configSerialized)), nil
}

var _ environment.EnvOpts = WithObservabilityConfig

func ObservabilityConfigFromContext(ctx context.Context) string {
	if e, ok := ctx.Value(observabilityCfgEnvKey{}).(string); ok {
		return e
	}
	panic("no observability config found in the context, make sure you properly configured the env opts using WithObservabilityConfig")
}
