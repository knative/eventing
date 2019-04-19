package channelwatcher

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type WatchHandlerFunc func(context.Context, client.Client, types.NamespacedName) error

type reconciler struct {
	client  client.Client
	logger  *zap.Logger
	handler WatchHandlerFunc
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := logging.WithLogger(context.TODO(), r.logger.With(zap.Any("request", req)))
	logging.FromContext(ctx).Info("New update for channel.")
	if err := r.handler(ctx, r.client, req.NamespacedName); err != nil {
		logging.FromContext(ctx).Error("WatchHandlerFunc returned error", zap.Error(err))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func New(mgr manager.Manager, logger *zap.Logger, watchHandler WatchHandlerFunc) error {
	c, err := controller.New("ChannelWatcher", mgr, controller.Options{
		Reconciler: &reconciler{
			client:  mgr.GetClient(),
			logger:  logger,
			handler: watchHandler,
		},
	})
	if err != nil {
		logger.Error("Unable to create controller for channelwatcher.", zap.Error(err))
		return err
	}

	// Watch Channels.
	err = c.Watch(&source.Kind{
		Type: &v1alpha1.Channel{},
	}, &handler.EnqueueRequestForObject{})
	if err != nil {
		logger.Error("Unable to watch Channels.", zap.Error(err), zap.Any("type", &v1alpha1.Channel{}))
		return err
	}
	return nil
}
