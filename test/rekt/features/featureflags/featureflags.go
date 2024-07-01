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

package featureflags

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/feature"

	apifeature "knative.dev/eventing/pkg/apis/feature"
)

const (
	istioFeatureFlagName = "istio"
)

func TransportEncryptionPermissiveOrStrict() feature.ShouldRun {
	return func(ctx context.Context, t feature.T) (feature.PrerequisiteResult, error) {
		flags, err := getFeatureFlags(ctx, "config-features")
		if err != nil {
			return feature.PrerequisiteResult{}, err
		}

		return feature.PrerequisiteResult{
			ShouldRun: flags.IsPermissiveTransportEncryption() || flags.IsStrictTransportEncryption(),
			Reason:    flags.String(),
		}, nil
	}
}

func TransportEncryptionStrict() feature.ShouldRun {
	return func(ctx context.Context, t feature.T) (feature.PrerequisiteResult, error) {
		flags, err := getFeatureFlags(ctx, "config-features")
		if err != nil {
			return feature.PrerequisiteResult{}, err
		}

		return feature.PrerequisiteResult{
			ShouldRun: flags.IsStrictTransportEncryption(),
			Reason:    flags.String(),
		}, nil
	}
}

func AuthenticationOIDCEnabled() feature.ShouldRun {
	return func(ctx context.Context, t feature.T) (feature.PrerequisiteResult, error) {
		flags, err := getFeatureFlags(ctx, "config-features")
		if err != nil {
			return feature.PrerequisiteResult{}, err
		}

		return feature.PrerequisiteResult{
			ShouldRun: flags.IsOIDCAuthentication(),
			Reason:    flags.String(),
		}, nil
	}
}

func CrossEventLinksEnabled() feature.ShouldRun {
	return func(ctx context.Context, t feature.T) (feature.PrerequisiteResult, error) {
		flags, err := getFeatureFlags(ctx, "config-features")
		if err != nil {
			return feature.PrerequisiteResult{}, err
		}

		return feature.PrerequisiteResult{
			ShouldRun: flags.IsCrossNamespaceEventLinks(),
			Reason:    flags.String(),
		}, nil
	}
}

func IstioDisabled() feature.ShouldRun {
	return func(ctx context.Context, t feature.T) (feature.PrerequisiteResult, error) {
		flags, err := getFeatureFlags(ctx, "config-features")
		if err != nil {
			return feature.PrerequisiteResult{}, err
		}

		return feature.PrerequisiteResult{
			ShouldRun: !flags.IsEnabled(istioFeatureFlagName),
			Reason:    flags.String(),
		}, nil
	}
}

func getFeatureFlags(ctx context.Context, cmName string) (apifeature.Flags, error) {
	ns := system.Namespace()
	cm, err := kubeclient.Get(ctx).
		CoreV1().
		ConfigMaps(ns).
		Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get cm %s/%s: %s", ns, cmName, err)
	}

	return apifeature.NewFlagsConfigFromConfigMap(cm)
}
