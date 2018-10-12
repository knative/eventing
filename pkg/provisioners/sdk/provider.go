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
	"encoding/json"
	"fmt"
	"github.com/knative/pkg/logging"
	"github.com/mattbaird/jsonpatch"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type KnativeReconciler interface {
	Reconcile(ctx context.Context, object runtime.Object) (runtime.Object, error)
	InjectClient(c client.Client) error
	InjectConfig(c *rest.Config) error
}

type reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder

	provider Provider
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

type Provider struct {
	AgentName string
	// Parent is a resource kind to reconcile with empty content. i.e. &v1.Parent{}
	Parent runtime.Object
	// Owns are dependent resources owned by the parent for which changes to
	// those resources cause the Parent to be re-reconciled. This is a list of
	// resources of kind with empty content. i.e. [&v1.Child{}]
	Owns []runtime.Object

	Reconciler KnativeReconciler
}

// ProvideController returns a controller for controller-runtime.
func (p *Provider) ProvideController(mgr manager.Manager) (controller.Controller, error) {
	// Setup a new controller to Reconcile Subscriptions.
	c, err := controller.New(p.AgentName, mgr, controller.Options{
		Reconciler: &reconciler{
			recorder: mgr.GetRecorder(p.AgentName),
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch Parent events and enqueue Parent object key.
	if err := c.Watch(&source.Kind{Type: p.Parent}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// Watch and enqueue for owning obj key.
	for _, t := range p.Owns {
		if err := c.Watch(&source.Kind{Type: t},
			&handler.EnqueueRequestForOwner{OwnerType: p.Parent, IsController: true}); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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

	if chg, err := hasChanged(ctx, original, obj); err != nil || !chg {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
		return reconcile.Result{}, err
	} else if _, err := r.updateStatus(request, obj); err != nil {
		logger.Warnf("Failed to update %s status: %v", r.provider.Parent.GetObjectKind(), err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	r.provider.Reconciler.InjectClient(c)
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	r.provider.Reconciler.InjectConfig(c)
	return err
}

func (r *reconciler) updateStatus(request reconcile.Request, object runtime.Object) (runtime.Object, error) {
	freshObj := r.provider.Parent.DeepCopyObject()
	if err := r.client.Get(context.TODO(), request.NamespacedName, freshObj); err != nil {
		return nil, err
	}

	fresh := NewReflectedStatusAccessor(freshObj)
	org := NewReflectedStatusAccessor(object)

	fresh.SetStatus(org.GetStatus())

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Source resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	if err := r.client.Update(context.TODO(), freshObj); err != nil {
		return nil, err
	}
	return freshObj, nil
}

// Not worth fully duck typing since there's no shared schema.
type hasSpec struct {
	Spec json.RawMessage `json:"spec"`
}

func getSpecJSON(crd runtime.Object) ([]byte, error) {
	b, err := json.Marshal(crd)
	if err != nil {
		return nil, err
	}
	hs := hasSpec{}
	if err := json.Unmarshal(b, &hs); err != nil {
		return nil, err
	}
	return []byte(hs.Spec), nil
}

func hasChanged(ctx context.Context, old, new runtime.Object) (bool, error) {
	if old == nil {
		return true, nil
	}
	logger := logging.FromContext(ctx)

	oldSpecJSON, err := getSpecJSON(old)
	if err != nil {
		logger.Error("Failed to get Spec JSON for old", zap.Error(err))
		return false, err
	}
	newSpecJSON, err := getSpecJSON(new)
	if err != nil {
		logger.Error("Failed to get Spec JSON for new", zap.Error(err))
		return false, err
	}

	specPatches, err := jsonpatch.CreatePatch(oldSpecJSON, newSpecJSON)
	if err != nil {
		fmt.Printf("Error creating JSON patch:%v", err)
		return false, err
	}
	if len(specPatches) == 0 {
		return false, nil
	}
	specPatchesJSON, err := json.Marshal(specPatches)
	if err != nil {
		logger.Error("Failed to marshal spec patches", zap.Error(err))
		return false, err
	}
	logger.Infof("Specs differ:\n%+v\n", string(specPatchesJSON))
	return true, nil
}

// Conditions is the interface for a Resource that implements the getter and
// setter for accessing a Condition collection.
// +k8s:deepcopy-gen=true
type StatusAccessor interface {
	GetStatus() interface{}
	SetStatus(interface{})
}

// NewReflectedConditionsAccessor uses reflection to return a ConditionsAccessor
// to access the field called "Conditions".
func NewReflectedStatusAccessor(object interface{}) StatusAccessor {
	objectValue := reflect.Indirect(reflect.ValueOf(object))

	// If object is not a struct, don't even try to use it.
	if objectValue.Kind() != reflect.Struct {
		return nil
	}

	statusField := objectValue.FieldByName("Status")

	if statusField.IsValid() && statusField.CanInterface() && statusField.CanSet() {
		if _, ok := statusField.Interface().(interface{}); ok {
			return &reflectedStatusAccessor{
				status: statusField,
			}
		}
	}
	return nil
}

// reflectedConditionsAccessor is an internal wrapper object to act as the
// ConditionsAccessor for status objects that do not implement ConditionsAccessor
// directly, but do expose the field using the "Conditions" field name.
type reflectedStatusAccessor struct {
	status reflect.Value
}

// GetConditions uses reflection to return Conditions from the held status object.
func (r *reflectedStatusAccessor) GetStatus() interface{} {
	if r != nil && r.status.IsValid() && r.status.CanInterface() {
		if status, ok := r.status.Interface().(interface{}); ok {
			return status
		}
	}
	return nil
}

// SetConditions uses reflection to set Conditions on the held status object.
func (r *reflectedStatusAccessor) SetStatus(status interface{}) {
	if r != nil && r.status.IsValid() && r.status.CanSet() {
		r.status.Set(reflect.ValueOf(status))
	}
}
