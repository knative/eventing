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

package resolve

import (
	"context"
	"fmt"
	"net/url"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler/names"
	duckapis "github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
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
func ResourceInterface(dynamicClient dynamic.Interface, namespace string, ref *corev1.ObjectReference) (dynamic.ResourceInterface, error) {
	rc := dynamicClient.Resource(duckapis.KindToResource(ref.GroupVersionKind()))

	if rc == nil {
		return nil, fmt.Errorf("failed to create dynamic client resource")
	}
	return rc.Namespace(namespace), nil
}

// ObjectReference resolves an object based on an ObjectReference.
func ObjectReference(ctx context.Context, dynamicClient dynamic.Interface, namespace string, ref *corev1.ObjectReference) (duck.Marshalable, error) {
	resourceClient, err := ResourceInterface(dynamicClient, namespace, ref)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to create dynamic resource client", zap.Error(err))
		return nil, err
	}

	return resourceClient.Get(ref.Name, metav1.GetOptions{})
}

// ResolveSubscriberSpec resolves the Spec.Call object. If it's an
// ObjectReference will resolve the object and treat it as an Addressable. If
// it's DNSName then it's used as is.
// TODO: Once Service Routes, etc. support Callable, use that.
func SubscriberSpec(ctx context.Context, dynamicClient dynamic.Interface, namespace string, s *v1alpha1.SubscriberSpec) (string, error) {
	if isNilOrEmptySubscriber(s) {
		return "", nil
	}
	if s.DNSName != nil && *s.DNSName != "" {
		return *s.DNSName, nil
	}

	obj, err := ObjectReference(ctx, dynamicClient, namespace, s.Ref)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to fetch SubscriberSpec target",
			zap.Error(err),
			zap.Any("subscriberSpec.Ref", s.Ref))
		return "", err
	}

	// K8s services are special cased. They can be called, even though they do not satisfy the
	// Callable interface.
	if s.Ref != nil && s.Ref.APIVersion == "v1" && s.Ref.Kind == "Service" {
		// This Service must exist because ObjectReference did not return an error.
		return DomainToURL(names.ServiceHostName(s.Ref.Name, namespace)), nil
	}

	t := duckv1alpha1.AddressableType{}
	if err = duck.FromUnstructured(obj, &t); err == nil {
		if t.Status.Address != nil {
			return DomainToURL(t.Status.Address.Hostname), nil
		}
	}

	legacy := duckv1alpha1.LegacyTarget{}
	if err = duck.FromUnstructured(obj, &legacy); err == nil {
		if legacy.Status.DomainInternal != "" {
			return DomainToURL(legacy.Status.DomainInternal), nil
		}
	}

	return "", fmt.Errorf("status does not contain address")
}

func isNilOrEmptySubscriber(sub *v1alpha1.SubscriberSpec) bool {
	return sub == nil || equality.Semantic.DeepEqual(sub, &v1alpha1.SubscriberSpec{})
}
