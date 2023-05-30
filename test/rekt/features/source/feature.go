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

package source

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
)

type Condition func(s *duckv1.Source) (bool, error)

func ExpectHTTPSSink(gvr schema.GroupVersionResource, name string) feature.StepFn {
	return waitFor(gvr, name, func(s *duckv1.Source) (bool, error) {
		if strings.EqualFold(s.Status.SinkURI.Scheme, "https") {
			return true, nil
		}
		return false, debugErr(s, gvr)
	})
}

func ExpectCACerts(gvr schema.GroupVersionResource, name string) feature.StepFn {
	return waitFor(gvr, name, func(s *duckv1.Source) (bool, error) {
		if s.Status.SinkCACerts != nil && *s.Status.SinkCACerts != "" {
			if err := validateCACerts(s.Status.SinkCACerts); err != nil {
				return false, err
			}
			return true, nil
		}

		return false, debugErr(s, gvr)
	})
}

func waitFor(gvr schema.GroupVersionResource, name string, cond Condition) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		k8s.IsReady(gvr, name)(ctx, t) // Precondition

		namespace := environment.FromContext(ctx).Namespace()
		obj, err := dynamicclient.Get(ctx).Resource(gvr).
			Namespace(namespace).
			Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Failed to get %+v %s/%s: %w", gvr, namespace, name, err)
			return
		}

		s := &duckv1.Source{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, s); err != nil {
			t.Fatalf("Error DefaultUnstructured.Dynamiconverter. %v", err)
		}

		if ok, err := cond(s); !ok {
			t.Error(err)
			return
		}
	}
}

func validateCACerts(CACert *string) error {
	block, err := pem.Decode([]byte(*CACert))
	if err != nil && block == nil {
		return apis.ErrInvalidValue("CA Cert provided is invalid", "sinkCACerts")
	}
	if block.Type != "CERTIFICATE" {
		return apis.ErrInvalidValue("CA Cert provided is not a certificate", "sinkCACerts")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return apis.ErrInvalidValue("CA Cert provided is invalid", "sinkCACerts")
	}
	return nil
}

func debugErr(s *duckv1.Source, gvr schema.GroupVersionResource) error {
	bytes, _ := json.MarshalIndent(s, "", "  ")
	return fmt.Errorf("Source (%+v) %s doesn't have HTTPS sink in status\n%s", gvr, s.Name, string(bytes))
}
