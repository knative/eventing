package reconciler

import (
	"context"
	"fmt"
	"reflect"

	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	AddFinalizerFailed = "AddFinalizerFailed"
	Reconciled         = "Reconciled"
	UpdateStatusFailed = "UpdateStatusFailed"
)

type ReconciledResource interface {
	metav1.Object
	runtime.Object
}

type EventingReconciler interface {
	ReconcileResource(context.Context, ReconciledResource, record.EventRecorder) (bool, reconcile.Result, error)
	SetClient(c client.Client) error
	GetNewReconcileObject() ReconciledResource
}

type FilterFunc func(context.Context, ReconciledResource, record.EventRecorder) bool
type FinalizerFunc func(context.Context, ReconciledResource, record.EventRecorder) error

type ReconcilerBuilder interface {
	Build() reconcile.Reconciler
	WithFinalizer(string, FinalizerFunc) ReconcilerBuilder
	WithFilter(FilterFunc) ReconcilerBuilder
	WithLogger(*zap.Logger) ReconcilerBuilder
	WithRecorder(record.EventRecorder) ReconcilerBuilder
}

type ReconcilerBuilderImpl struct {
	r *reconciler
}

var _ ReconcilerBuilder = &ReconcilerBuilderImpl{}

func (b *ReconcilerBuilderImpl) Build() reconcile.Reconciler {
	return b.r
}

func (b *ReconcilerBuilderImpl) WithFinalizer(finalizerName string, finalizer FinalizerFunc) ReconcilerBuilder {
	b.r.finalizerName = finalizerName
	b.r.finalizer = finalizer
	return b
}

func (b *ReconcilerBuilderImpl) WithFilter(filter FilterFunc) ReconcilerBuilder {
	b.r.filter = filter
	return b
}

func (b *ReconcilerBuilderImpl) WithLogger(logger *zap.Logger) ReconcilerBuilder {
	b.r.logger = logger
	return b
}

func (b *ReconcilerBuilderImpl) WithRecorder(recorder record.EventRecorder) ReconcilerBuilder {
	b.r.recorder = recorder
	return b
}

func NewBuilder(er EventingReconciler) ReconcilerBuilder {
	return &ReconcilerBuilderImpl{r: &reconciler{EventingReconciler: er}}
}

type reconciler struct {
	finalizerName string
	client        client.Client
	recorder      record.EventRecorder
	logger        *zap.Logger
	EventingReconciler
	filter    FilterFunc
	finalizer FinalizerFunc
}

var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	r.SetClient(c)
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	obj := r.GetNewReconcileObject()
	recObjTypeName := reflect.TypeOf(obj).Name() //TODO: Investigate why .Name() returns "". Need Name and not namespace.name

	// TODO: Fix the code for cases where logger, recorder, or client are not set.
	ctx := logging.WithLogger(context.TODO(), r.logger.With(zap.Any("request", request)))

	if r.filter != nil && !r.filter(ctx, obj, r.recorder) {
		logging.FromContext(ctx).Debug(fmt.Sprintf("Not reconciling %s as ShouldReconcile() returned false", recObjTypeName))
		return reconcile.Result{}, nil
	}

	logging.FromContext(ctx).Debug(fmt.Sprintf("Reconciling %s", recObjTypeName))

	if err := r.client.Get(ctx, request.NamespacedName, obj); err != nil {
		if errors.IsNotFound(err) {
			logging.FromContext(ctx).Error(fmt.Sprintf("Could not find %s", recObjTypeName))
			return reconcile.Result{}, nil
		}
		logging.FromContext(ctx).Error(fmt.Sprintf("Error getting %s", recObjTypeName), zap.Error(err))
		return reconcile.Result{}, err
	}

	if obj.GetDeletionTimestamp() != nil {
		if r.finalizer != nil {
			if err := r.finalizer(ctx, obj, r.recorder); err != nil {
				logging.FromContext(ctx).Error("Finalizer func failed.", zap.Error(err))
				return reconcile.Result{}, err
			}
		}
		provisioners.RemoveFinalizer(obj, r.finalizerName)
		return reconcile.Result{}, nil
	}

	if r.finalizerName != "" {
		result := provisioners.AddFinalizer(obj, r.finalizerName)
		if result == provisioners.FinalizerAdded {
			if err := r.client.Update(ctx, obj); err != nil {
				logging.FromContext(ctx).Error(
					fmt.Sprintf("Error reconciling %s. Adding finalizer %s failed.", recObjTypeName, r.finalizerName),
					zap.Error(err),
				)
				eventFailedReason := recObjTypeName + AddFinalizerFailed
				r.recorder.Eventf(obj, corev1.EventTypeWarning, eventFailedReason, "%s reconciliation failed: %v", recObjTypeName, err)
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}
	}

	updateStatus, result, err := r.ReconcileResource(ctx, obj, r.recorder)

	if err != nil {
		logging.FromContext(ctx).Warn(fmt.Sprintf("Error reconciling %s", recObjTypeName), zap.Error(err))
	} else {
		logging.FromContext(ctx).Debug(fmt.Sprintf("%s reconciled", recObjTypeName))
		reason := recObjTypeName + Reconciled
		r.recorder.Eventf(obj, corev1.EventTypeNormal, reason, "%s reconciled: %q", recObjTypeName, obj.GetName())
	}

	if updateStatus {
		if updataStatusErr := r.client.Status().Update(ctx, obj); updataStatusErr != nil {
			logging.FromContext(ctx).Warn(fmt.Sprintf("Failed to update %s", recObjTypeName), zap.Error(updataStatusErr))
			reason := recObjTypeName + UpdateStatusFailed
			r.recorder.Eventf(obj, corev1.EventTypeWarning, reason, "Failed to update Subscription's status: %v", updataStatusErr)
			return reconcile.Result{}, updataStatusErr
		}
	}
	return result, err
}
