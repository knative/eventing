/*
Copyright 2018 The Knative Authors

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
package main

import (
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	flowsv1alpha1 "github.com/knative/eventing/pkg/apis/flows/v1alpha1"
	"github.com/knative/eventing/pkg/controller/eventing/subscription"
	"github.com/knative/eventing/pkg/controller/feed"
	"github.com/knative/eventing/pkg/controller/flow"
	"github.com/knative/pkg/configmap"
	"go.uber.org/zap"
	"strings"

	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"

	"github.com/knative/eventing/pkg/controller/eventtype"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

// SchemeFunc adds types to a Scheme.
type SchemeFunc func(*runtime.Scheme) error

// ProvideFunc adds a controller to a Manager.
type ProvideFunc func(manager.Manager) (controller.Controller, error)

// KnownProvideControllers is a list of controllers that will be used inside
// controller-runtime. Config is expecting the following values:
// - eventtype.feeds.knative.dev
// - feed.feeds.knative.dev
// - flow.flows.knative.dev
// - subscription.eventing.knative.dev
var KnownProvideControllers = map[string]ProvideFunc{
	"eventtype.feeds.knative.dev":       eventtype.ProvideController,
	"feed.feeds.knative.dev":            feed.ProvideController,
	"flow.flows.knative.dev":            flow.ProvideController,
	"subscription.eventing.knative.dev": subscription.ProvideController,
}

// controllerRuntimeStart runs controllers written for controller-runtime. It's
// intended to be called from main(). Any controllers migrated to use
// controller-runtime should move their initialization to this function.
func controllerRuntimeStart(logger *zap.SugaredLogger) error {
	logf.SetLogger(logf.ZapLogger(false))

	// Load the mapped configmap value, controller will not watch for changes.
	cm, err := configmap.Load("/etc/config-controllers")
	if err != nil {
		logger.Info("Error loading controller configuration: %v\nUsing defaults.", zap.Error(err))
	}

	// Setup a Manager
	mrg, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		return err
	}

	// Add custom types to this array to get them into the manager's scheme.
	schemeFuncs := []SchemeFunc{
		channelsv1alpha1.AddToScheme,
		feedsv1alpha1.AddToScheme,
		flowsv1alpha1.AddToScheme,
		istiov1alpha3.AddToScheme,
		eventingv1alpha1.AddToScheme,
	}
	for _, schemeFunc := range schemeFuncs {
		schemeFunc(mrg.GetScheme())
	}

	// Add each controller's ProvideController func to this list to have the
	// manager run it by config.
	providers := make([]ProvideFunc, 0, len(KnownProvideControllers))
	if len(cm) == 0 {
		for _, f := range KnownProvideControllers {
			providers = append(providers, f)
		}
	} else {
		for k, v := range cm {
			if strings.ToLower(v) == "true" {
				if f, ok := KnownProvideControllers[k]; !ok {
					logger.Infof("Failed to find a known controller for %q.", k)
				} else {
					providers = append(providers, f)
				}
			} else {
				logger.Infof("Skipping controller for %q.", k)
			}
		}
	}

	for _, provider := range providers {
		if _, err := provider(mrg); err != nil {
			return err
		}
	}

	return mrg.Start(signals.SetupSignalHandler())
}
