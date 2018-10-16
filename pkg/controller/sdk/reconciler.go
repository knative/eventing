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

package sdk

import (
	"context"
	"github.com/knative/pkg/logging"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder

	provider Provider
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &Reconciler{}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx)

	logger.Infof("Reconciling %s %v", r.provider.Parent.GetObjectKind(), request)

	obj := r.provider.Parent.DeepCopyObject()

	err := r.client.Get(context.TODO(), request.NamespacedName, obj)

	if errors.IsNotFound(err) {
		logger.Errorf("could not find %s %v\n", r.provider.Parent.GetObjectKind(), request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		logger.Errorf("could not fetch %s %v for %+v\n", r.provider.Parent.GetObjectKind(), err, request)
		return reconcile.Result{}, err
	}

	original := obj.DeepCopyObject()

	// Reconcile this copy of the Source and then write back any status
	// updates regardless of whether the reconcile error out.
	obj, err = r.provider.Reconciler.Reconcile(ctx, obj)
	if err != nil {
		logger.Warnf("Failed to reconcile %s: %v", r.provider.Parent.GetObjectKind(), err)
	}

	if chg, err := r.statusHasChanged(ctx, original, obj); err != nil || !chg {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
		return reconcile.Result{}, err
	} else if _, err := r.updateStatus(ctx, request, obj); err != nil {
		logger.Warnf("Failed to update %s status: %v", r.provider.Parent.GetObjectKind(), err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.client = c
	if r.provider.Reconciler != nil {
		r.provider.Reconciler.InjectClient(c)
	}
	return nil
}

func (r *Reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	if r.provider.Reconciler != nil {
		r.provider.Reconciler.InjectConfig(c)
	}
	return err
}

func (r *Reconciler) statusHasChanged(ctx context.Context, old, new runtime.Object) (bool, error) {
	if old == nil {
		return true, nil
	}

	o := NewReflectedStatusAccessor(old)
	n := NewReflectedStatusAccessor(new)

	oStatus := o.GetStatus()
	nStatus := n.GetStatus()

	if equality.Semantic.DeepEqual(oStatus, nStatus) {
		return false, nil
	}

	return true, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, request reconcile.Request, object runtime.Object) (runtime.Object, error) {
	freshObj := r.provider.Parent.DeepCopyObject()
	if err := r.client.Get(ctx, request.NamespacedName, freshObj); err != nil {
		return nil, err
	}

	fresh := NewReflectedStatusAccessor(freshObj)
	org := NewReflectedStatusAccessor(object)

	fresh.SetStatus(org.GetStatus())

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Source resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	if err := r.client.Update(ctx, freshObj); err != nil {
		return nil, err
	}
	return freshObj, nil
}
