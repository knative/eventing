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
	"encoding/json"
	//	"fmt"
	//	"net/url"

	//	"github.com/knative/pkg/kmeta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1alpha1 "github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	//	corev1 "k8s.io/api/core/v1"
	//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"k8s.io/apimachinery/pkg/runtime"
	//	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NewChannel returns a Channel CRD for the specififed GVK and Spec
func NewChannel(name string, p *v1alpha1.Pipeline) (*unstructured.Unstructured, error) {
	// Set the name of the resource we're creating as well as the namespace, etc.
	new := p.DeepCopy()
	template := new.Spec.ChannelTemplate
	template.Name = name
	template.Namespace = p.Namespace
	// TODO: Set the owner ref, etc.
	raw, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	err = json.Unmarshal(raw, u)
	if err != nil {
		return nil, err
	}
	return u, nil
}
