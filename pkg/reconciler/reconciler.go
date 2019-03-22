package reconciler

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// Reason text for event recorder. making it public so that reconcilers in other packages can consume it esp while tesing
const (
	AddFinalizerFailed    = "AddFinalizerFailed"
	RemoveFinalizerFailed = "RemoveFinalizerFailed"
	Reconciled            = "Reconciled"
	UpdateStatusFailed    = "UpdateStatusFailed"
	ReconcileFailed       = "ReconcileFailed"
)

// ReconciledResource must be implemented by the type that is being reconciled
type ReconciledResource interface {
	metav1.Object
	runtime.Object
}

// EventingReconciler must be implemented by any reconciler that wants to inherit this common reonciler pattern
type EventingReconciler interface {
	ReconcileResource(context.Context, ReconciledResource, record.EventRecorder) (bool, reconcile.Result, error)
	GetNewReconcileObject() ReconciledResource
	inject.Client
}

// Finalizer must be implemented by reconcilers that need to clean external resources when the resource being watched is deleted
type Finalizer interface {
	OnDelete(context.Context, ReconciledResource, record.EventRecorder) error
}

// Filter must be implemented by reconcilers that want to skip reconciling objects based on some conditions
// Please prefer using a predicate when creating the watch. However watch predicates get called on the object that is being watched
// and not on the object that is being reconciled. In such a case implement filter.
// Example: 1. Watch K8s Service but reconcile a Trigger that owns it -> Use Filter
// 2. Watch Trigger and reconcile Trigger --> Use predicate while registering watch
type Filter interface {
	ShouldReconcile(context.Context, ReconciledResource, record.EventRecorder) bool
}

// RequestModifier must be implemented if the incoming request needs to be modified before reconciling starts.
// This should be used sparingly and usually identifies issues with controller runtime that needs to be followed up with controller-runtime team.
// This is done to support reconcilers that reconcile cluster-scoped resources based on watch on a namespace-scoped resource
// Workaround until https://github.com/kubernetes-sigs/controller-runtime/issues/228 is fix and controller-runtime updated
// TODO: remove after updating controller-runtime
type RequestModifier interface {
	Modify(*reconcile.Request)
}

// RequestModifierFunc simplifies writing Modify functions by implementing RequestModifier
type RequestModifierFunc func(*reconcile.Request)

// Modify simplifies injecting Modify functions using RequestModifierFunc
func (f RequestModifierFunc) Modify(r *reconcile.Request) {
	f(r)
}

type option func(*reconciler) error

// EnableFinalizer option indicates explicitly that the reconciler implements Finalizer interface
func EnableFinalizer(finalizerName string) option {
	return func(r *reconciler) error {
		if finalizerName == "" {
			return errors.New("finalizerName is empty. Please provide a non-empty vlue")
		}
		f, ok := r.EventingReconciler.(Finalizer)
		if !ok {
			return errors.New("EventingReconciler doesn't implement Finalizer interface")
		}
		r.finalizerName = finalizerName
		r.finalizer = f
		return nil
	}
}

// EnableFilter option indicates explicitly that the reconciler implements Filter interface
func EnableFilter() option {
	return func(r *reconciler) error {
		f, ok := r.EventingReconciler.(Filter)
		if !ok {
			return errors.New("EventingReconciler doesn't implement Filter interface")
		}
		r.filter = f
		return nil
	}
}

// ModifyRequest option indicates explicitly that the reconciler needs to modify requests before reconciling
func ModifyRequest(rm RequestModifier) option {
	return func(r *reconciler) error {
		r.requestModifier = rm
		return nil
	}
}

// EnableConfigInjection option indicates explicitly that the reconciler implements inject.Config interface
// as defined in sigs.k8s.io/controller-runtime/pkg/runtime
func EnableConfigInjection() option {
	return func(r *reconciler) error {
		ic, ok := r.EventingReconciler.(inject.Config)
		if !ok {
			return errors.New("EventingReconciler doesn't implement inject.Config interface")
		}
		r.injectConfig = ic
		return nil
	}
}

// New creates a new generic reconciler that encapsulates the provided EventingReconciler and enables functionalities based on provided options
func New(er EventingReconciler, logger *zap.Logger, recorder record.EventRecorder, opts ...option) (reconcile.Reconciler, error) {
	if er == nil {
		return nil, errors.New("EventingReconciler is nil")
	}
	if logger == nil {
		return nil, errors.New("logger is nil")
	}
	if recorder == nil {
		return nil, errors.New("recorder is nil")
	}
	r := &reconciler{
		EventingReconciler: er,
		logger:             logger,
		recorder:           recorder,
	}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return r, err
		}
	}
	return r, nil
}

type reconciler struct {
	finalizerName string
	client        client.Client
	recorder      record.EventRecorder
	logger        *zap.Logger
	EventingReconciler
	injectConfig    inject.Config
	filter          Filter
	finalizer       Finalizer
	requestModifier RequestModifier
}

var _ reconcile.Reconciler = &reconciler{}
var _ inject.Client = &reconciler{}
var _ inject.Config = &reconciler{}

// inject.Client impl
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return r.EventingReconciler.InjectClient(c)
}

// inject.Config impl
func (r *reconciler) InjectConfig(c *rest.Config) error {
	if r.injectConfig != nil {
		return r.injectConfig.InjectConfig(c)
	}
	return nil
}

// reconcile.Reconciler impl
// Reconcile enforces the following pattern
// 1. Modify the request if applicable
// 2. Get the object using client and hangle edge cases
// 3. Check if the object should be reconciled
// 4. Check if it is deletion call and handle finalezers accordingly
// 5. Check if finalizers need to be added and add them correctly
// 6. Let the EventingReconciler reconcile the object in-memory
// 7. Update the status of the object using client apis, if applicable.
// 8. Standardizes logging and event recording
// 9. Abstracts updates and finalizer handling to avoid unnecessary bugs and corner cases.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	obj := r.GetNewReconcileObject()
	recObjTypeName := reflect.TypeOf(obj).Elem().Name()

	// This is done to support reconcilers that reconcile cluster-scoped resources based on watch on a namespace-scoped resource
	// TODO: Workaround until https://github.com/kubernetes-sigs/controller-runtime/issues/228 is fix and controller-runtime updated
	if r.requestModifier != nil {
		r.requestModifier.Modify(&request)
	}

	ctx := logging.WithLogger(context.TODO(), r.logger.With(zap.Any("request", request), zap.Any("ResourceKind", recObjTypeName)))
	logger := logging.FromContext(ctx)

	logger.Debug("Reconciling.")

	if err := r.client.Get(ctx, request.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error("Could not find object.")
			return reconcile.Result{}, nil
		}
		logger.Error("Error in client.Get()", zap.Error(err))
		return reconcile.Result{}, err
	}

	// First check if the reconciler implement Filter.ShouldReconcile() and that the object should be reconciled or not
	if r.filter != nil && !r.filter.ShouldReconcile(ctx, obj, r.recorder) {
		logger.Debug("Skip reconciling as ShouldReconcile() returned false")
		return reconcile.Result{}, nil
	}

	// If the object is being deleted then run OnDelete functions and remove finalizers
	if !obj.GetDeletionTimestamp().IsZero() {
		if r.finalizer != nil {
			if err := r.finalizer.OnDelete(ctx, obj, r.recorder); err != nil {
				r.reportRemoveFinalizerFailed(ctx, obj, err)
				return reconcile.Result{}, err
			}
		}
		provisioners.RemoveFinalizer(obj, r.finalizerName)
		if err := r.client.Update(ctx, obj); err != nil {
			r.reportRemoveFinalizerFailed(ctx, obj, err)
			return reconcile.Result{}, err
		}
		// Return as there is nothing else the the reconciler can do
		r.reportObjectReconciled(ctx, obj)
		return reconcile.Result{}, nil
	}

	if r.finalizerName != "" {
		result := provisioners.AddFinalizer(obj, r.finalizerName)
		if result == provisioners.FinalizerAdded {
			// If this update succeeds then proceed with rest of reconcilliation as obj will have the
			// returned updated obj with new ResourceVersion
			if err := r.client.Update(ctx, obj); err != nil {
				logger.Error("Reconcile failed while adding finalizer", zap.Any("FinalizerName", r.finalizerName), zap.Error(err))
				r.recorder.Eventf(obj, corev1.EventTypeWarning, AddFinalizerFailed, "Reconcile failed: %s", err)
				return reconcile.Result{}, err
			}
		}
	}

	updateStatus, result, err := r.ReconcileResource(ctx, obj, r.recorder)

	if err != nil {
		logger.Warn(fmt.Sprintf("Error reconciling"), zap.Error(err))
		r.recorder.Eventf(obj, corev1.EventTypeWarning, ReconcileFailed, "Reconcile failed: %s", err)
	} else {
		r.reportObjectReconciled(ctx, obj)
	}
	if updateStatus {
		if updataStatusErr := r.client.Status().Update(ctx, obj); updataStatusErr != nil {
			logger.Error("Failed to update status.", zap.Error(updataStatusErr))
			r.recorder.Eventf(obj, corev1.EventTypeWarning, UpdateStatusFailed, "Failed to update status: %s", updataStatusErr)
			return reconcile.Result{}, updataStatusErr
		}
	}

	return result, err
}

func (r *reconciler) reportObjectReconciled(ctx context.Context, obj ReconciledResource) {
	logging.FromContext(ctx).Debug("Reconciled.")
	r.recorder.Eventf(obj, corev1.EventTypeNormal, Reconciled, "Reconciled.")
}

func (r *reconciler) reportRemoveFinalizerFailed(ctx context.Context, obj ReconciledResource, err error) {
	logging.FromContext(ctx).Error("Reconcile failed while removing finalizer", zap.Any("FinalizerName", r.finalizerName), zap.Error(err))
	r.recorder.Eventf(obj, corev1.EventTypeWarning, RemoveFinalizerFailed, "Reconcile failed: %s", err)
}
