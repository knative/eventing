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
	"bytes"
	"context"
	"text/template"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"
)

const (
	ConfigMapName = "config-kreference-mapping"
)

type MappingResolver struct {
	logger    *zap.SugaredLogger
	templates map[schema.GroupVersionKind]*template.Template
	tracker   tracker.Interface
}

type MappingResolverTemplateValues struct {
	Name            string
	Namespace       string
	SystemNamespace string
	UID             types.UID
}

func NewMappingResolver(ctx context.Context, cmw configmap.Watcher, t tracker.Interface) *MappingResolver {
	resolver := MappingResolver{
		logger:  logging.FromContext(ctx),
		tracker: t,
	}
	cmw.Watch(ConfigMapName, resolver.updateFromConfigMap)

	return &resolver
}

func (mr *MappingResolver) MappingURIFromObjectReference(ctx context.Context, ref *corev1.ObjectReference) (bool, *apis.URL, error) {
	mr.logger.Infow("resolving reference", zap.Any("ref", ref))

	if !feature.FromContext(ctx).IsEnabled(feature.KReferenceMapping) {
		mr.logger.Infow("kreference mapping feature not enabled")
		return false, nil, nil // not handled.
	}

	// TODO: needs the parent object.
	//if err := mr.tracker.TrackReference(tracker.Reference{
	//	APIVersion: ref.APIVersion,
	//	Kind:       ref.Kind,
	//	Namespace:  ref.Namespace,
	//	Name:       ref.Name,
	//}, parent); err != nil {
	//	return nil, apierrs.NewNotFound(gvr.GroupResource(), ref.Name)
	//}

	if mr.templates == nil {
		mr.logger.Infow("reference not handled", zap.Any("gvk", ref.GroupVersionKind()))
		return false, nil, nil
	}

	tmpl, ok := mr.templates[ref.GroupVersionKind()]
	if !ok {
		mr.logger.Infow("reference not handled", zap.Any("gvk", ref.GroupVersionKind()))
		return false, nil, nil
	}

	data := MappingResolverTemplateValues{
		Name:            ref.Name,
		Namespace:       ref.Namespace,
		UID:             ref.UID,
		SystemNamespace: system.Namespace(),
	}

	buf := bytes.Buffer{}
	if err := tmpl.Execute(&buf, data); err != nil {
		// Configuration error
		return true, nil, err
	}

	url, err := apis.ParseURL(buf.String())
	if err != nil {
		// Configuration error
		return true, nil, err
	}

	return true, url, nil
}

func (mr *MappingResolver) updateFromConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		return
	}
	mr.logger.Infow("loading kreference mapping configmap")

	mr.templates = make(map[schema.GroupVersionKind]*template.Template)

	for key, stmpl := range cfg.Data {
		if key == "_example" {
			continue
		}
		mr.logger.Infow("processing mapping", zap.String("key", key), zap.String("template", stmpl))

		gvk, gk := schema.ParseKindArg(key)

		if gvk == nil {
			// Try <kind>.<version> (core k8s API)
			if gk.Group == "" {
				// Wrong key format
				mr.logger.Warnw("failed to parse kreference mapping key. Must be of the form <kind>.<version>(.<group>)?", zap.String("key", key))
				continue
			}

			gvk = &schema.GroupVersionKind{
				Kind:    gk.Kind,
				Version: gk.Group,
			}
		}

		tmpl, err := template.New(key).Parse(stmpl)
		if err != nil {
			mr.logger.Warnw("failed to parse kreference mapping template", zap.String("template", stmpl), zap.Error(err))
			continue
		}

		mr.templates[*gvk] = tmpl
		mr.logger.Infow("mapping template loaded", zap.String("template", stmpl), zap.Error(err))
	}
}
