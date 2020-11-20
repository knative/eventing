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

package manifest

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

type Manifest interface {
	// Either updates or creates all resources in the manifest
	ApplyAll() error
	// Updates or creates a particular resource
	Apply(*unstructured.Unstructured) error
	// Deletes all resources in the manifest
	DeleteAll() error
	// Deletes a particular resource
	Delete(spec *unstructured.Unstructured) error
	// Returns a deep copy of the matching resource read from the file
	Find(apiVersion string, kind string, name string) *unstructured.Unstructured
	// Returns the resource fetched from the api server, nil if not found
	Get(spec *unstructured.Unstructured) (*unstructured.Unstructured, error)
	// Returns a deep copy of all resources in the manifest
	DeepCopyResources() []unstructured.Unstructured
	// ResourceNames is a convenient list of all the resource names in the manifest
	ResourceNames() []string
	// References is a convenient list of all resources in the manifest as object references
	References() []corev1.ObjectReference
}

type YamlManifest struct {
	client    dynamic.Interface
	resources []unstructured.Unstructured
}

var _ Manifest = &YamlManifest{}

func NewYamlManifest(pathname string, recursive bool, client dynamic.Interface) (Manifest, error) {
	klog.Info("Reading YAML filepath: ", pathname, " recursive: ", recursive)
	resources, err := Parse(pathname, recursive)
	if err != nil {
		return nil, err
	}
	return &YamlManifest{resources: resources, client: client}, nil
}

func (f *YamlManifest) ApplyAll() error {
	for _, spec := range f.resources {
		if err := f.Apply(&spec); err != nil {
			return err
		}
	}
	return nil
}

func (f *YamlManifest) Apply(spec *unstructured.Unstructured) error {
	current, err := f.Get(spec)
	if err != nil {
		return err
	}
	if current == nil {
		klog.Info("Creating type ", spec.GroupVersionKind(), " name ", spec.GetName())
		gvr, _ := meta.UnsafeGuessKindToResource(spec.GroupVersionKind())
		if _, err := f.client.Resource(gvr).Namespace(spec.GetNamespace()).Create(context.Background(), spec, v1.CreateOptions{}); err != nil {
			return err
		}
	} else {
		// Update existing one
		if UpdateChanged(spec.UnstructuredContent(), current.UnstructuredContent()) {
			klog.Info("Updating type ", spec.GroupVersionKind(), " name ", spec.GetName())

			gvr, _ := meta.UnsafeGuessKindToResource(spec.GroupVersionKind())
			if _, err = f.client.Resource(gvr).Namespace(current.GetNamespace()).Update(context.Background(), current, v1.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *YamlManifest) DeleteAll() error {
	a := make([]unstructured.Unstructured, len(f.resources))
	copy(a, f.resources)
	// we want to delete in reverse order
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
	for _, spec := range a {
		if err := f.Delete(&spec); err != nil {
			return err
		}
	}
	return nil
}

func (f *YamlManifest) Delete(spec *unstructured.Unstructured) error {
	current, err := f.Get(spec)
	if current == nil && err == nil {
		return nil
	}
	klog.Info("Deleting type ", spec.GroupVersionKind(), " name ", spec.GetName())
	gvr, _ := meta.UnsafeGuessKindToResource(spec.GroupVersionKind())
	if err := f.client.Resource(gvr).Namespace(spec.GetNamespace()).Delete(context.Background(), spec.GetName(), v1.DeleteOptions{}); err != nil {
		// ignore GC race conditions triggered by owner references
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (f *YamlManifest) Get(spec *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	gvr, _ := meta.UnsafeGuessKindToResource(spec.GroupVersionKind())
	result, err := f.client.Resource(gvr).Namespace(spec.GetNamespace()).Get(context.Background(), spec.GetName(), v1.GetOptions{})
	if err != nil {
		result = nil
		if errors.IsNotFound(err) {
			err = nil
		}
	}
	return result, err
}

func (f *YamlManifest) Find(apiVersion string, kind string, name string) *unstructured.Unstructured {
	for _, spec := range f.resources {
		if spec.GetAPIVersion() == apiVersion &&
			spec.GetKind() == kind &&
			spec.GetName() == name {
			return spec.DeepCopy()
		}
	}
	return nil
}

func (f *YamlManifest) DeepCopyResources() []unstructured.Unstructured {
	result := make([]unstructured.Unstructured, len(f.resources))
	for i, spec := range f.resources {
		result[i] = *spec.DeepCopy()
	}
	return result
}

func (f *YamlManifest) ResourceNames() []string {
	var names []string
	for _, spec := range f.resources {
		names = append(names, fmt.Sprintf("%s/%s (%s)", spec.GetNamespace(), spec.GetName(), spec.GroupVersionKind()))
	}
	return names
}

func (f *YamlManifest) References() []corev1.ObjectReference {
	var refs []corev1.ObjectReference
	for _, spec := range f.resources {

		ref := corev1.ObjectReference{
			Name:       spec.GetName(),
			Namespace:  spec.GetNamespace(),
			APIVersion: spec.GetAPIVersion(),
			Kind:       spec.GetKind(),
		}

		refs = append(refs, ref)
	}
	return refs
}

// We need to preserve the top-level target keys, specifically
// 'metadata.resourceVersion', 'spec.clusterIP', and any existing
// entries in a ConfigMap's 'data' field. So we only overwrite fields
// set in our src resource.
func UpdateChanged(src, tgt map[string]interface{}) bool {
	changed := false
	for k, v := range src {
		if v, ok := v.(map[string]interface{}); ok {
			if tgt[k] == nil {
				tgt[k], changed = v, true
			} else if UpdateChanged(v, tgt[k].(map[string]interface{})) {
				// This could be an issue if a field in a nested src
				// map doesn't overwrite its corresponding tgt
				changed = true
			}
			continue
		}
		if !equality.Semantic.DeepEqual(v, tgt[k]) {
			tgt[k], changed = v, true
		}
	}
	return changed
}
