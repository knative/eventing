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
	"log"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/kmeta"
)

// Internal version of ChannelTemplateSpec that includes ObjectMeta so that
// we can easily create new Channels off of it.
type triggerImporterSpecInternal struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the Spec to use for each channel created. Passed
	// in verbatim to the Channel CRD as Spec section.
	// +optional
	Spec *runtime.RawExtension `json:"spec,omitempty"`
}

func NewImporter(t v1alpha1.Trigger, name string, importer v1alpha1.TriggerImporterSpec) (*unstructured.Unstructured, error) {
	// Set the name of the resource we're creating as well as the namespace, etc.
	template := triggerImporterSpecInternal{
		TypeMeta: metav1.TypeMeta{
			APIVersion: importer.APIVersion,
			Kind:       importer.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(&t),
			},
			Name:      name,
			Namespace: t.Namespace,
		},
	}

	spec, err := injectSink(t, *importer.Spec)
	if err != nil {
		return nil, err
	}
	template.Spec = &spec

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

func injectSink(t v1alpha1.Trigger, spec runtime.RawExtension) (runtime.RawExtension, error) {
	m := make(map[string]interface{})
	if err := json.Unmarshal(spec.Raw, &m); err != nil {
		return runtime.RawExtension{}, err
	}
	m["sink"] = v1.ObjectReference{
		APIVersion: t.GetGroupVersionKind().GroupVersion().String(),
		Kind:       t.GetGroupVersionKind().Kind,
		Name:       t.Name,
	}
	log.Printf("injectSink***********: %+v", m)
	j, err := json.Marshal(m)
	if err != nil {
		return runtime.RawExtension{}, err
	}
	return runtime.RawExtension{
		Raw: j,
	}, nil
}
