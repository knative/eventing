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

package v1

import (
	"context"

	"github.com/google/go-cmp/cmp/cmpopts"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"

	"knative.dev/eventing/pkg/apis/config"
)

const (
	BrokerClassAnnotationKey = "eventing.knative.dev/broker.class"
)

func (b *Broker) Validate(ctx context.Context) *apis.FieldError {
	ctx = apis.WithinParent(ctx, b.ObjectMeta)
	cfg := config.FromContextOrDefaults(ctx)

	var errs *apis.FieldError
	withNS := determineNamespaceAllowance(ctx, cfg.Defaults)

	// Make sure a BrokerClassAnnotation exists
	if bc, ok := b.GetAnnotations()[BrokerClassAnnotationKey]; !ok || bc == "" {
		errs = errs.Also(apis.ErrMissingField(BrokerClassAnnotationKey))
	}

	// Further validation logic
	errs = errs.Also(b.Spec.Validate(withNS).ViaField("spec"))
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Broker)
		errs = errs.Also(b.CheckImmutableFields(ctx, original))
	}

	return errs
}

// Determine if the namespace allowance based on the given configuration
func determineNamespaceAllowance(ctx context.Context, brConfig *config.Defaults) context.Context {

	// If there is no configurations set, allow different namespace by default
	if brConfig == nil || (brConfig.NamespaceDefaultsConfig == nil && brConfig.ClusterDefaultConfig == nil) {
		return apis.AllowDifferentNamespace(ctx)
	}
	namespace := apis.ParentMeta(ctx).Namespace
	nsConfig := brConfig.NamespaceDefaultsConfig[namespace]

	// Check if the namespace disallows different namespace first
	if nsConfig == nil || nsConfig.DisallowDifferentNamespaceConfig == nil || !*nsConfig.DisallowDifferentNamespaceConfig {
		// If there is no namespace specific configuration or DisallowDifferentNamespaceConfig is false, check the cluster level configuration
		if brConfig.ClusterDefaultConfig == nil || brConfig.ClusterDefaultConfig.DisallowDifferentNamespaceConfig == nil || !*brConfig.ClusterDefaultConfig.DisallowDifferentNamespaceConfig {
			// If there is no cluster level configuration or DisallowDifferentNamespaceConfig in cluster level is false, allow different namespace
			return apis.AllowDifferentNamespace(ctx)
		}
	}
	// If we reach here, it means DisallowDifferentNamespaceConfig is true at either level, no need to explicitly disallow, just return the original context
	return ctx
}

func (bs *BrokerSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Validate the Config
	if bs.Config != nil {
		if ce := bs.Config.Validate(ctx); ce != nil {
			errs = errs.Also(ce.ViaField("config"))
		}
	}

	if bs.Delivery != nil {
		if de := bs.Delivery.Validate(ctx); de != nil {
			errs = errs.Also(de.ViaField("delivery"))
		}
	}
	return errs
}

func (b *Broker) CheckImmutableFields(ctx context.Context, original *Broker) *apis.FieldError {
	if original == nil {
		return nil
	}

	// Only Delivery options are mutable.
	ignoreArguments := cmpopts.IgnoreFields(BrokerSpec{}, "Delivery")
	if diff, err := kmp.ShortDiff(original.Spec, b.Spec, ignoreArguments); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff Broker",
			Paths:   []string{"spec"},
			Details: err.Error(),
		}
	} else if diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}

	// Make sure you can't change the class annotation.
	if diff, _ := kmp.ShortDiff(original.GetAnnotations()[BrokerClassAnnotationKey], b.GetAnnotations()[BrokerClassAnnotationKey]); diff != "" {
		return &apis.FieldError{
			Message: "Immutable annotations changed (-old +new)",
			Paths:   []string{"annotations"},
			Details: diff,
		}
	}

	return nil
}
