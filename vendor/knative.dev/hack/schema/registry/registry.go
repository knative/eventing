/*
Copyright 2021 The Knative Authors

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

package registry

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
)

type Registry struct {
	// easy for now
	kinds map[string]reflect.Type
}

var r = &Registry{
	kinds: map[string]reflect.Type{},
}

// GVKable indicates that a particular type can return metadata about the Kind.
type GVKable interface {
	// GetGroupVersionKind returns a GroupVersionKind. The name is chosen
	// to avoid collision with TypeMeta's GroupVersionKind() method.
	// See: https://issues.k8s.io/3030
	GetGroupVersionKind() schema.GroupVersionKind
}

func Register(obj GVKable) {
	t := reflect.TypeOf(obj)
	gvk := obj.GetGroupVersionKind()
	r.kinds[gvk.Kind] = t.Elem()
}

func Kinds() []string {
	kinds := make([]string, 0)
	for k := range r.kinds {
		kinds = append(kinds, k)
	}
	return kinds
}

func TypeFor(kind string) reflect.Type {
	return r.kinds[kind]
}
