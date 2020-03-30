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
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/adapter/v2"
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
	Name string `envconfig:"NAME" required:"true"`

	Config Config `envconfig:"K_SOURCE_CONFIG" required:"true"`
}

type apiServerAdapter struct {
	namespace string
	ce        cloudevents.Client
	logger    *zap.SugaredLogger

	config Config

	k8s    dynamic.Interface
	source string // TODO: who dis?
	name   string // TODO: who dis?
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
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

	a := &apiServerAdapter{
		k8s:    client,
		ce:     ceClient,
		source: cfg.Host,
		name:   env.Name,
		config: env.Config,

		logger: logger,
	}
	return a
}

func (a *apiServerAdapter) Start(stopCh <-chan struct{}) error {
	// Local stop channel.
	stop := make(chan struct{})

	resyncPeriod := 10 * time.Hour

	var delegate cache.Store = &resourceDelegate{
		ce:     a.ce,
		source: a.source,
		logger: a.logger,
		ref:    a.config.EventMode == v1alpha2.ReferenceMode,
	}

	if a.config.ResourceOwner != nil {
		if a.config.ResourceOwner.APIVersion != nil && a.config.ResourceOwner.Kind != nil {
			delegate = &controllerFilter{
				apiVersion: *a.config.ResourceOwner.APIVersion,
				kind:       *a.config.ResourceOwner.Kind,
				delegate:   delegate,
			}
		}
	}

	for _, gvr := range a.config.Resources {
		lw := &cache.ListWatch{
			ListFunc:  asUnstructuredLister(a.k8s.Resource(gvr).Namespace(a.namespace).List, a.config.LabelSelector),
			WatchFunc: asUnstructuredWatcher(a.k8s.Resource(gvr).Namespace(a.namespace).Watch, a.config.LabelSelector),
		}

		reflector := cache.NewReflector(lw, &unstructured.Unstructured{}, delegate, resyncPeriod)
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
