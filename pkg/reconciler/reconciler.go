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

const (
	AddFinalizerFailed    = "AddFinalizerFailed"
	RemoveFinalizerFailed = "UpdateFinalizerFailed"
	Reconciled            = "Reconciled"
	UpdateStatusFailed    = "UpdateStatusFailed"
	ReconcileFailed       = "ReconcileFailed"
)

type ReconciledResource interface {
	metav1.Object
	runtime.Object
}

type EventingReconciler interface {
	ReconcileResource(context.Context, ReconciledResource, record.EventRecorder) (bool, reconcile.Result, error)
	GetNewReconcileObject() ReconciledResource
	inject.Client
}

type Finalizer interface {
	OnDelete(context.Context, ReconciledResource, record.EventRecorder) error
}

type Filter interface {
	ShouldReconcile(context.Context, ReconciledResource, record.EventRecorder) bool
}

type RequestModifier interface {
	Modify(*reconcile.Request)
}
type RequestModifierFunc func(*reconcile.Request)

func (f RequestModifierFunc) Modify(r *reconcile.Request) {
	f(r)
}

type option func(*reconciler) error

func EnableFinalizer(finalizerName string) option {
	return func(r *reconciler) error {
		if finalizerName == "" {
			return errors.New("finalizerName is empty. Please provide a non-empty vlue")
		}
		f, ok := r.EventingReconciler.(Finalizer)
		if !ok {
			return errors.New("EventingReconciler doesn't implement Finalizer")
		}
		r.finalizerName = finalizerName
		r.finalizer = f
		return nil
	}
}

func EnableFilter() option {
	return func(r *reconciler) error {
		f, ok := r.EventingReconciler.(Filter)
		if !ok {
			return errors.New("EventingReconciler doesn't implement Filter")
		}
		r.filter = f
		return nil
	}
}

func ModifyRequest(rm RequestModifier) option {
	return func(r *reconciler) error {
		r.requestModifier = rm
		return nil
	}
}

func Logger(logger *zap.Logger) option {
	return func(r *reconciler) error {
		r.logger = logger
		return nil
	}
}

func Recorder(recorder record.EventRecorder) option {
	return func(r *reconciler) error {
		r.recorder = recorder
		return nil
	}
}

// For this option to work the provided EventingReconciler must implement inject.Config interface
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
				logger.Error("Finalizer func failed.", zap.Error(err))
				return reconcile.Result{}, err
			}
		}
		provisioners.RemoveFinalizer(obj, r.finalizerName)
		if err := r.client.Update(ctx, obj); err != nil {
			logger.Error("Reconcile failed while removing finalizer", zap.Any("FinalizerName", r.finalizerName), zap.Error(err))
			reason := recObjTypeName + RemoveFinalizerFailed
			r.recorder.Eventf(obj, corev1.EventTypeWarning, reason, "%s reconciliation failed: %s", recObjTypeName, err)
			return reconcile.Result{}, err
		}
		// Return as there is nothing else the the reconciler can do
		r.reportObjectReconciled(obj, logger)
		return reconcile.Result{}, nil
	}

	if r.finalizerName != "" {
		result := provisioners.AddFinalizer(obj, r.finalizerName)
		if result == provisioners.FinalizerAdded {
			// If this update succeeds then proceed with rest of reconcilliation as obj will have the
			// returned updated obj with new ResourceVersion
			if err := r.client.Update(ctx, obj); err != nil {
				logger.Error("Reconcile failed while adding finalizer", zap.Any("FinalizerName", r.finalizerName), zap.Error(err))
				eventFailedReason := recObjTypeName + AddFinalizerFailed
				r.recorder.Eventf(obj, corev1.EventTypeWarning, eventFailedReason, "%s reconciliation failed: %s", recObjTypeName, err)
				return reconcile.Result{}, err
			}
		}
	}

	updateStatus, result, err := r.ReconcileResource(ctx, obj, r.recorder)

	if err != nil {
		logger.Warn(fmt.Sprintf("Error reconciling %s", recObjTypeName), zap.Error(err))
		reason := recObjTypeName + ReconcileFailed
		r.recorder.Eventf(obj, corev1.EventTypeWarning, reason, "%s reconcile failed: %s", recObjTypeName, err)
	} else {
		r.reportObjectReconciled(obj, logger)
	}
	if updateStatus {
		if updataStatusErr := r.client.Status().Update(ctx, obj); updataStatusErr != nil {
			logger.Error("Failed to update status.", zap.Error(updataStatusErr))
			reason := recObjTypeName + UpdateStatusFailed
			r.recorder.Eventf(obj, corev1.EventTypeWarning, reason, "Failed to update %s status: %s", recObjTypeName, updataStatusErr)
			return reconcile.Result{}, updataStatusErr
		}
	}

	return result, err
}

func (r *reconciler) reportObjectReconciled(obj ReconciledResource, logger *zap.Logger) {
	typeName := reflect.TypeOf(obj).Elem().Name()
	logger.Debug("Reconciled.")
	reason := typeName + Reconciled
	r.recorder.Eventf(obj, corev1.EventTypeNormal, reason, "%s reconciled.", typeName)
}
