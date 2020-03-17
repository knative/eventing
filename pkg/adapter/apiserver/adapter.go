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

package apiserver

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"

	"knative.dev/eventing/pkg/adapter"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

type StringList []string

// Decode splits list of strings separated by '|',
// overriding the default comma separator which is
// a valid label selector character.
func (s *StringList) Decode(value string) error {
	*s = strings.Split(value, ";")
	return nil
}

type envConfig struct {
	adapter.EnvConfig

	Mode            string     `envconfig:"MODE"`
	ApiVersion      StringList `split_words:"true" required:"true"`
	Kind            StringList `required:"true"`
	Controller      []bool     `required:"true"`
	LabelSelector   StringList `envconfig:"SELECTOR" required:"true"`
	OwnerApiVersion StringList `envconfig:"OWNER_API_VERSION" required:"true"`
	OwnerKind       StringList `envconfig:"OWNER_KIND" required:"true"`
	Name            string     `envconfig:"NAME" required:"true"`
}

const (
	// RefMode produces payloads of ObjectReference
	RefMode = "Ref"
	// ResourceMode produces payloads of ResourceEvent
	ResourceMode = "Resource"

	resourceGroup = "apiserversources.sources.knative.dev"
)

// GVRC is a combination of GroupVersionResource, Controller flag, LabelSelector and OwnerRef
type GVRC struct {
	GVR             schema.GroupVersionResource
	Controller      bool
	LabelSelector   string
	OwnerApiVersion string
	OwnerKind       string
}

type apiServerAdapter struct {
	namespace string
	ce        cloudevents.Client
	reporter  source.StatsReporter
	logger    *zap.SugaredLogger

	gvrcs    []GVRC
	k8s      dynamic.Interface
	source   string
	mode     string
	delegate eventDelegate
	name     string
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client, reporter source.StatsReporter) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		logger.Fatal("error building kubeconfig", zap.Error(err))
	}

	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		logger.Fatal("error building dynamic client", zap.Error(err))
	}

	gvrcs := []GVRC(nil)

	for i, apiVersion := range env.ApiVersion {
		kind := env.Kind[i]
		controlled := env.Controller[i]
		selector := env.LabelSelector[i]
		ownerApiVersion := env.OwnerApiVersion[i]
		ownerKind := env.OwnerKind[i]

		gv, err := schema.ParseGroupVersion(apiVersion)
		if err != nil {
			logger.Fatal("error parsing APIVersion", zap.Error(err))
		}
		// TODO: pass down the resource and the kind so we do not have to guess.
		gvr, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Kind: kind, Group: gv.Group, Version: gv.Version})
		gvrcs = append(gvrcs, GVRC{
			GVR:             gvr,
			Controller:      controlled,
			LabelSelector:   selector,
			OwnerApiVersion: ownerApiVersion,
			OwnerKind:       ownerKind,
		})
	}

	mode := env.Mode
	switch env.Mode {
	case ResourceMode, RefMode:
		// ok
	default:
		logger.Warn("unknown mode ", mode)
		mode = RefMode
		logger.Warn("defaulting mode to ", mode)
	}

	a := &apiServerAdapter{
		k8s:       client,
		ce:        ceClient,
		source:    cfg.Host,
		logger:    logger,
		gvrcs:     gvrcs,
		namespace: env.Namespace,
		mode:      mode,
		reporter:  reporter,
		name:      env.Name,
	}
	return a
}

type eventDelegate interface {
	cache.Store
	addControllerWatch(gvr schema.GroupVersionResource)
}

func (a *apiServerAdapter) Start(stopCh <-chan struct{}) error {
	// Local stop channel.
	stop := make(chan struct{})

	resyncPeriod := time.Duration(10 * time.Hour)
	var d eventDelegate
	switch a.mode {
	case ResourceMode:
		d = &resource{
			ce:        a.ce,
			source:    a.source,
			logger:    a.logger,
			reporter:  a.reporter,
			namespace: a.namespace,
			name:      a.name,
		}

	case RefMode:
		d = &ref{
			ce:        a.ce,
			source:    a.source,
			logger:    a.logger,
			reporter:  a.reporter,
			namespace: a.namespace,
			name:      a.name,
		}

	default:
		return fmt.Errorf("mode %q not understood", a.mode)
	}

	for _, gvrc := range a.gvrcs {
		lw := &cache.ListWatch{
			ListFunc:  asUnstructuredLister(a.k8s.Resource(gvrc.GVR).Namespace(a.namespace).List, gvrc.LabelSelector),
			WatchFunc: asUnstructuredWatcher(a.k8s.Resource(gvrc.GVR).Namespace(a.namespace).Watch, gvrc.LabelSelector),
		}

		if gvrc.Controller {
			d.addControllerWatch(gvrc.GVR)
		}

		var store cache.Store
		if gvrc.OwnerApiVersion != "" || gvrc.OwnerKind != "" {
			store = &controller{
				apiVersion: gvrc.OwnerApiVersion,
				kind:       gvrc.OwnerKind,
				delegate:   store,
			}
		} else {
			store = d
		}

		reflector := cache.NewReflector(lw, &unstructured.Unstructured{}, store, resyncPeriod)
		go reflector.Run(stop)
	}

	<-stopCh
	stop <- struct{}{}
	return nil
}

type unstructuredLister func(metav1.ListOptions) (*unstructured.UnstructuredList, error)

func asUnstructuredLister(ulist unstructuredLister, selector string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		if selector != "" && opts.LabelSelector == "" {
			opts.LabelSelector = selector
		}
		ul, err := ulist(opts)
		if err != nil {
			return nil, err
		}
		return ul, nil
	}
}

func asUnstructuredWatcher(wf cache.WatchFunc, selector string) cache.WatchFunc {
	return func(lo metav1.ListOptions) (watch.Interface, error) {
		if selector != "" && lo.LabelSelector == "" {
			lo.LabelSelector = selector
		}
		return wf(lo)
	}
}
