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

package broker

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

// GetLoggingConfig will get config from a specific namespace
func GetLoggingConfig(ctx context.Context, namespace, loggingConfigMapName string) (*logging.Config, error) {
	loggingConfigMap, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(namespace).Get(ctx, loggingConfigMapName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return logging.NewConfigFromMap(nil)
	} else if err != nil {
		return nil, err
	}
	return logging.NewConfigFromConfigMap(loggingConfigMap)
}
