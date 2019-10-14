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
	"fmt"
	"net/url"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"knative.dev/eventing/pkg/reconciler/names"
	duckapis "knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"

	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/logging"
)

// DomainToURL converts a domain into an HTTP URL.
func DomainToURL(domain string) string {
	u := url.URL{
		Scheme: "http",
		Host:   domain,
		Path:   "/",
	}
	return u.String()
}

// ResourceInterface creates a resource interface for the given ObjectReference.
func ResourceInterface(dynamicClient dynamic.Interface, namespace string, gvk schema.GroupVersionKind) (dynamic.ResourceInterface, error) {
	rc := dynamicClient.Resource(duckapis.KindToResource(gvk))

	if rc == nil {
		return nil, fmt.Errorf("failed to create dynamic client resource")
	}
	return rc.Namespace(namespace), nil
}

// ObjectReference resolves an object based on an ObjectReference.
func ObjectReference(ctx context.Context, dynamicClient dynamic.Interface, namespace string, ref *corev1.ObjectReference) (duck.Marshalable, error) {
	resourceClient, err := ResourceInterface(dynamicClient, namespace, ref.GroupVersionKind())
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to create dynamic resource client", zap.Error(err))
		return nil, err
	}

	return resourceClient.Get(ref.Name, metav1.GetOptions{})
}

// SubscriberSpec resolves the SubscriberSpec object. If it's an ObjectReference, it will resolve the
// object and treat it as an Addressable. If it's a DNSName, then it's used as is.
func SubscriberSpec(ctx context.Context, dynamicClient dynamic.Interface, namespace string, s *v1alpha1.SubscriberSpec, addressableTracker ListableTracker, track Track) (string, error) {
	if isNilOrEmptySubscriber(s) {
		return "", nil
	}
	if s.URI != nil && *s.URI != "" {
		return *s.URI, nil
	}
	if s.DeprecatedDNSName != nil && *s.DeprecatedDNSName != "" {
		return *s.DeprecatedDNSName, nil
	}

	if s.Ref == nil {
		return "", nil
	}

	if err := track(*s.Ref); err != nil {
		return "", fmt.Errorf("unable to track the reference: %v", err)
	}

	// K8s services are special cased. They can be called, even though they do not satisfy the
	// Addressable interface. Note that it is important to return here as we wouldn't be able to marshall it to an
	// Addressable, thus the type assertion below would fail.
	if s.Ref.APIVersion == "v1" && s.Ref.Kind == "Service" {
		// Check that the service exists by querying the API server. This is a special case, as we cannot use the
		// addressable lister.
		_, err := ObjectReference(ctx, dynamicClient, namespace, s.Ref)
		if err != nil {
			logging.FromContext(ctx).Warn("Failed to fetch SubscriberSpec target service", zap.Any("subscriberSpec.Ref", s.Ref), zap.Error(err))
			return "", err
		}
		// This Service must exist because ObjectReference did not return an error.
		return DomainToURL(names.ServiceHostName(s.Ref.Name, namespace)), nil
	}

	lister, err := addressableTracker.ListerFor(*s.Ref)
	if err != nil {
		logging.FromContext(ctx).Error(fmt.Sprintf("Error getting lister for ObjecRef: %s/%s", namespace, s.Ref.Name), zap.Error(err))
	}

	a, err := lister.ByNamespace(namespace).Get(s.Ref.Name)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to fetch SubscriberSpec target", zap.Any("subscriberSpec.Ref", s.Ref), zap.Error(err))
		return "", err
	}

	addressable, ok := a.(*duckv1alpha1.AddressableType)
	if !ok {
		logging.FromContext(ctx).Error(fmt.Sprintf("Failed to convert to Addressable Object: %s/%s", namespace, s.Ref.Name), zap.Error(err))
		return "", errors.New("object is not addressable")
	}
	if addressable.Status.Address != nil {
		url := addressable.Status.Address.GetURL()
		return url.String(), nil
	}
	return "", errors.New("status does not contain address")
}

func isNilOrEmptySubscriber(sub *v1alpha1.SubscriberSpec) bool {
	return sub == nil || equality.Semantic.DeepEqual(sub, &v1alpha1.SubscriberSpec{})
}
