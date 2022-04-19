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

package resolver

import (
	"context"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"
)

// NewURIResolver constructs a new URIResolver with context and a callback
// for a given listableType (Listable) passed to the URIResolver's tracker.
func NewURIResolver(ctx context.Context, cmw configmap.Watcher, t tracker.Interface) *resolver.URIResolver {
	mr := NewMappingResolver(ctx, cmw, t)

	return resolver.NewURIResolverFromTracker(ctx, t, mr.MappingURIFromObjectReference)
}
