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

package crd

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	crdreconciler "knative.dev/pkg/client/injection/apiextensions/reconciler/apiextensions/v1/customresourcedefinition"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"knative.dev/eventing/pkg/reconciler/source/duck"
)

type runningController struct {
	controller *controller.Impl
	cancel     context.CancelFunc
}

// Reconciler implements controller.Reconciler for Source CRDs resources.
type Reconciler struct {
	ogctx context.Context
	ogcmw configmap.Watcher

	// lock guards controllers
	lock sync.RWMutex

	// controllers keeps a map for GVR to dynamically created controllers.
	controllers map[schema.GroupVersionResource]runningController
}

// Check that our Reconciler implements crdreconciler.Interface
var _ crdreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, crd *v1.CustomResourceDefinition) pkgreconciler.Event {
	// The reconciliation process is as follows:
	// 	1. Resolve GVR and GVK from a particular Source CRD (i.e., those labeled with duck.knative.dev/source = "true")
	//  2. Dynamically create a controller for it, if not present already. Such controller is in charge of reconciling
	//     duckv1.Source resources with that particular GVR..

	gvr, gvk, err := r.resolveGroupVersions(ctx, crd)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error while resolving GVR and GVK", zap.String("CRD", crd.Name), zap.Error(err))
		return err
	}

	if !crd.DeletionTimestamp.IsZero() {
		// We are intentionally not setting up a finalizer on the CRD.
		// This might leave unnecessary dynamic controllers running.
		// This is a best effort to try to clean them up.
		// Note that without a finalizer there is no guarantee we will be called.
		r.deleteController(ctx, gvr)
		return nil
	}

	err = r.reconcileController(ctx, crd, gvr, gvk)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error while reconciling controller", zap.String("GVR", gvr.String()), zap.String("GVK", gvk.String()), zap.Error(err))
		return err
	}

	return nil
}

func (r *Reconciler) resolveGroupVersions(ctx context.Context, crd *v1.CustomResourceDefinition) (*schema.GroupVersionResource, *schema.GroupVersionKind, error) {
	var gvr *schema.GroupVersionResource
	var gvk *schema.GroupVersionKind
	for _, v := range crd.Spec.Versions {
		if !v.Served {
			continue
		}
		gvr = &schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  v.Name,
			Resource: crd.Spec.Names.Plural,
		}

		gvk = &schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: v.Name,
			Kind:    crd.Spec.Names.Kind,
		}

	}
	if gvr == nil || gvk == nil {
		return nil, nil, fmt.Errorf("unable to find GVR or GVK for %s", crd.Name)
	}
	return gvr, gvk, nil
}

func (r *Reconciler) deleteController(ctx context.Context, gvr *schema.GroupVersionResource) {
	r.lock.RLock()
	_, found := r.controllers[*gvr]
	r.lock.RUnlock()
	if found {
		r.lock.Lock()
		// Now that we grabbed the write lock, check that nobody deleted it already.
		rc, found := r.controllers[*gvr]
		if found {
			logging.FromContext(ctx).Infow("Stopping Source Duck Controller", zap.String("GVR", gvr.String()))
			rc.cancel()
			delete(r.controllers, *gvr)
		}
		r.lock.Unlock()
	}
}

func (r *Reconciler) reconcileController(ctx context.Context, crd *v1.CustomResourceDefinition, gvr *schema.GroupVersionResource, gvk *schema.GroupVersionKind) error {
	r.lock.RLock()
	_, found := r.controllers[*gvr]
	r.lock.RUnlock()
	if found {
		return nil
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	// Now that we grabbed the write lock, check that nobody has created the controller.
	_, found = r.controllers[*gvr]
	if found {
		return nil
	}

	// Source Duck controller constructor
	sdc := duck.NewController(crd.Name, *gvr, *gvk)
	if sdc == nil {
		logging.FromContext(ctx).Errorw("Source Duck Controller is nil.", zap.String("GVR", gvr.String()), zap.String("GVK", gvk.String()))
		return nil
	}

	// Source Duck controller context
	sdctx, cancel := context.WithCancel(r.ogctx)
	// Source Duck controller instantiation
	sd := sdc(sdctx, r.ogcmw)

	rc := runningController{
		controller: sd,
		cancel:     cancel,
	}
	r.controllers[*gvr] = rc

	logging.FromContext(ctx).Infow("Starting Source Duck Controller", zap.String("GVR", gvr.String()), zap.String("GVK", gvk.String()))
	go func(c *controller.Impl) {
		if c != nil {
			if err := c.Run(controller.DefaultThreadsPerController, sdctx.Done()); err != nil {
				logging.FromContext(ctx).Errorw("Unable to start Source Duck Controller", zap.String("GVR", gvr.String()), zap.String("GVK", gvk.String()))
			}
		}
	}(rc.controller)
	return nil
}
