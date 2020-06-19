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
	"encoding/json"
	"fmt"
	"net/url"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	duckapis "knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
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
func ObjectReference(ctx context.Context, dynamicClient dynamic.Interface, namespace string, ref *corev1.ObjectReference) (json.Marshaler, error) {
	resourceClient, err := ResourceInterface(dynamicClient, namespace, ref.GroupVersionKind())
	if err != nil {
		logging.FromContext(ctx).Warnw("Failed to create dynamic resource client", zap.Error(err))
		return nil, err
	}

	return resourceClient.Get(ctx, ref.Name, metav1.GetOptions{})
}
