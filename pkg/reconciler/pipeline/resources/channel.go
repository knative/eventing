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

package resources

import (
	//	"fmt"
	//	"net/url"

	//	"github.com/knative/pkg/kmeta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	messagingv1alpha1 "github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	//	corev1 "k8s.io/api/core/v1"
	//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	//	"k8s.io/apimachinery/pkg/runtime/schema"
)

func getObject(version, kind, name string, spec *runtime.RawExtension) *unstructured.Unstructured {
	nonNilSpec := spec
	if spec == nil {
		nonNilSpec = "{}"
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": version,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": nonNilSpec,
		},
	}
}

// NewChannel returns a Channel CRD for the specififed GVK and Spec
func NewChannel(name string, cts *messagingv1alpha1.ChannelTemplateSpec) *unstructured.Unstructured {
	return getObject(cts.ChannelCRD.APIVersion, cts.ChannelCRD.Kind, name, cts.Spec)
}
