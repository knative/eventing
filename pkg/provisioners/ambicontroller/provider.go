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

package controller

import (
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "heartbeats-provisioner-controller"
)

type reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a Subscription controller.
func ProvideController(mgr manager.Manager) (controller.Controller, error) {
	// Setup a new controller to Reconcile Subscriptions.
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch Source events and enqueue Source object key.
	if err := c.Watch(&source.Kind{Type: &v1alpha1.Source{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// Watch Channels and enqueue owning Source key.
	if err := c.Watch(&source.Kind{Type: &v1alpha1.Channel{}},
		&handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Source{}, IsController: true}); err != nil {
		return nil, err
	}

	// Watch Pods and enqueue owning Source key.
	if err := c.Watch(&source.Kind{Type: &corev1.Pod{}},
		&handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Source{}, IsController: true}); err != nil {
		return nil, err
	}

	return c, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx)

	logger.Infof("Reconciling source %v", request)
	source := &v1alpha1.Source{}
	err := r.client.Get(context.TODO(), request.NamespacedName, source)

	if errors.IsNotFound(err) {
		logger.Errorf("could not find source %v\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		logger.Errorf("could not fetch Source %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	if source.Spec.Provisioner.Ref.Name != provisionerName {
		logger.Errorf("heartbeats skipping source %s, provisioned by %s\n", source.Name, source.Spec.Provisioner.Ref.Name)
		return reconcile.Result{}, nil
	}

	original := source.DeepCopy()

	// Reconcile this copy of the Source and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(ctx, source)
	if err != nil {
		logger.Warnf("Failed to reconcile source: %v", err)
	}
	if equality.Semantic.DeepEqual(original.Status, source.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := r.updateStatus(source); err != nil {
		logger.Warnf("Failed to update source status: %v", err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}
