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
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	flowsv1alpha1 "github.com/knative/eventing/pkg/apis/flows/v1alpha1"
	"github.com/knative/eventing/pkg/controller/feed"
	"github.com/knative/eventing/pkg/controller/flow"

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

// controllerRuntimeStart runs controllers written for controller-runtime. It's
// intended to be called from main(). Any controllers migrated to use
// controller-runtime should move their initialization to this function.
func controllerRuntimeStart() error {
	logf.SetLogger(logf.ZapLogger(false))

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
	}
	for _, schemeFunc := range schemeFuncs {
		schemeFunc(mrg.GetScheme())
	}

	// Add each controller's ProvideController func to this list to have the
	// manager run it.
	providers := []ProvideFunc{
		eventtype.ProvideController,
		feed.ProvideController,
		flow.ProvideController,
	}

	for _, provider := range providers {
		if _, err := provider(mrg); err != nil {
			return err
		}
	}

	return mrg.Start(signals.SetupSignalHandler())
}
