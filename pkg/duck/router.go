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

package duck

import (
	"context"
	"errors"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
)

// ResolveRouterURI resolves the Router object.
func ResolveRouterURI(ctx context.Context, dynamicClient dynamic.Interface, namespace string, s *eventingduck.RoutableSpec, track Track) (string, error) {
	if s == nil || s.Ref == nil {
		return "", nil
	}

	obj, err := ObjectReference(ctx, dynamicClient, namespace, s.Ref)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to fetch Router target",
			zap.Error(err),
			zap.Any("RouterSpec.Ref", s.Ref))
		return "", err
	}

	// if err = track(*s.Ref); err != nil {
	// 	return "", fmt.Errorf("unable to track the reference: %v", err)
	// }

	// K8s services are special cased. They can be called, even though they do not satisfy the
	// Callable interface.
	if s.Ref != nil && s.Ref.APIVersion == "v1" && s.Ref.Kind == "Service" {
		// This Service must exist because ObjectReference did not return an error.
		return DomainToURL(names.ServiceHostName(s.Ref.Name, namespace)), nil
	}

	t := duckv1alpha1.AddressableType{}
	if err = duck.FromUnstructured(obj, &t); err == nil {
		if t.Status.Address != nil {
			url := t.Status.Address.GetURL()
			return url.String(), nil
		}
	}

	legacy := duckv1alpha1.LegacyTarget{}
	if err = duck.FromUnstructured(obj, &legacy); err == nil {
		if legacy.Status.DomainInternal != "" {
			return DomainToURL(legacy.Status.DomainInternal), nil
		}
	}

	return "", errors.New("status does not contain address")
}
