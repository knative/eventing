// +build !ignore_autogenerated

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

// Code generated by reconciler-gen. DO NOT EDIT.

package hackhack

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	client "knative.dev/eventing/pkg/legacyclient/injection/client"
	containersource "knative.dev/eventing/pkg/legacyclient/injection/informers/legacysources/v1alpha1/containersource"
	controller "knative.dev/pkg/controller"
	logging "knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
)

const (
	controllerAgentName = "containersource-controller"
	finalizerName       = "containersource"
)

func NewImpl(ctx context.Context, r *Reconciler) *controller.Impl {
	logger := logging.FromContext(ctx)

	impl := controller.NewImpl(r, logger, "containersources")

	injectionInformer := containersource.Get(ctx)

	r.Core = Core{
		Client:  client.Get(ctx),
		Lister:  injectionInformer.Lister(),
		Tracker: tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx)),
		Recorder: record.NewBroadcaster().NewRecorder(
			scheme.Scheme, v1.EventSource{Component: controllerAgentName}),
		FinalizerName: finalizerName,
		Reconciler:    r,
	}

	logger.Info("Setting up core event handlers")
	injectionInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	return impl
}
