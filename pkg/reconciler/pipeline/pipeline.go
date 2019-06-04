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

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	duckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions/messaging/v1alpha1"
	eventinglisters "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/messaging/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/pipeline/resources"
	duckroot "github.com/knative/pkg/apis"
	duckapis "github.com/knative/pkg/apis/duck"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Pipelines"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "pipeline-controller"

	reconciled         = "Reconciled"
	reconcileFailed    = "ReconcileFailed"
	updateStatusFailed = "UpdateStatusFailed"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	pipelineLister     listers.PipelineLister
	channelLister      eventinglisters.ChannelLister
	subscriptionLister eventinglisters.SubscriptionLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	pipelineInformer informers.PipelineInformer,
	channelInformer eventinginformers.ChannelInformer,
	subscriptionInformer eventinginformers.SubscriptionInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:               reconciler.NewBase(opt, controllerAgentName),
		pipelineLister:     pipelineInformer.Lister(),
		channelLister:      channelInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	pipelineInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Register handlers for Channel/Subscriptions that are owned by Pipeline, so that
	// we get notified if they change.
	channelInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Channel")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Subscription")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}

// Reconcile compares the actual state with the desired, and attempts to
// reconcile the two. It then updates the Status block of the Pipeline resource
// with the current Status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logging.FromContext(ctx).Error("reconciling", zap.String("key", key))
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the Pipeline resource with this namespace/name
	original, err := r.pipelineLister.Pipelines(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("Pipeline key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	pipeline := original.DeepCopy()

	// Reconcile this copy of the Pipeline and then write back any status
	// updates regardless of whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, pipeline)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Pipeline", zap.Error(reconcileErr))
		r.Recorder.Eventf(pipeline, corev1.EventTypeWarning, reconcileFailed, "Pipeline reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("Successfully reconciled Pipeline")
		r.Recorder.Eventf(pipeline, corev1.EventTypeNormal, reconciled, "Pipeline reconciled")
	}

	if _, updateStatusErr := r.updateStatus(ctx, pipeline.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Error updating Pipeline status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(pipeline, corev1.EventTypeWarning, updateStatusFailed, "Failed to update pipeline status: %s", key)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return reconcileErr
}

func pipelineSubscriptionName(pipelineName string, step int) string {
	return fmt.Sprintf("%s-kn-pipeline-%d", pipelineName, step)
}

func (r *Reconciler) reconcile(ctx context.Context, p *v1alpha1.Pipeline) error {
	p.Status.InitializeConditions()

	// Reconciling pipeline is pretty straightforward, it does the following things:
	// 1. Create a channel fronting the whole pipeline
	// 2. For each of the Steps, create a Subscription to the previous Channel
	//    (hence the first step above for the first step in the "steps"), where the Subscriber points to the
	//    Step, and create intermediate channel for feeding the Reply to (if we allow Reply to be something else
	//    than channel, we could just (optionally) feed it directly to the following subscription.
	// 3. Rinse and repeat step #2 above for each Step in the list
	// 4. If there's a Reply, then the last Subscription will be configured to send the reply to that.
	if p.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	// Convert the object into runtime.Object so we can grab the schema.ObjectKind and create the correct interface client
	obj := p.Spec.ChannelTemplate.DeepCopyObject()

	channelResourceInterface := r.DynamicClientSet.Resource(duckroot.KindToResource(obj.GetObjectKind().GroupVersionKind())).Namespace(p.Namespace)

	if channelResourceInterface == nil {
		msg := fmt.Sprintf("Unable to create dynamic client for: %+v", p.Spec.ChannelTemplate)
		logging.FromContext(ctx).Error(msg)
		errors.New(msg)
	}

	channels := []*duckv1alpha1.Channelable{}
	for i := 0; i < len(p.Spec.Steps); i++ {
		ingressChannelName := resources.PipelineChannelName(p.Name, i)
		c, err := r.reconcileChannel(ctx, ingressChannelName, channelResourceInterface, p)
		if err != nil {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to reconcile Channel Object: %s/%s", p.Namespace, ingressChannelName), zap.Error(err))
			return err

		}
		// Convert to Channel duck so that we can treat all Channels the same.
		channelable := &duckv1alpha1.Channelable{}
		err = duckapis.FromUnstructured(c, channelable)
		if err != nil {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", p.Namespace, ingressChannelName), zap.Error(err))
			return err

		}
		channels = append(channels, channelable)
		logging.FromContext(ctx).Info(fmt.Sprintf("Reconciled Channel Object: %s/%s %+v", p.Namespace, ingressChannelName, c))
	}
	p.Status.PropagateChannelStatuses(channels)

	subs := []*eventingv1alpha1.Subscription{}
	for i := 0; i < len(p.Spec.Steps); i++ {
		sub, err := r.reconcileSubscription(ctx, i, p)
		if err != nil {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to reconcile Subscription Object for step: %d", i), zap.Error(err))
			return err
		}
		subs = append(subs, sub)
		logging.FromContext(ctx).Info(fmt.Sprintf("Reconciled Subscription Object for step: %d: %+v", i, sub))
	}
	p.Status.PropagateSubscriptionStatuses(subs)

	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Pipeline) (*v1alpha1.Pipeline, error) {
	p, err := r.pipelineLister.Pipelines(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(p.Status, desired.Status) {
		return p, nil
	}

	// Don't modify the informers copy.
	existing := p.DeepCopy()
	existing.Status = desired.Status

	return r.EventingClientSet.MessagingV1alpha1().Pipelines(desired.Namespace).UpdateStatus(existing)
}

func (r *Reconciler) reconcileChannel(ctx context.Context, channelName string, channelResourceInterface dynamic.ResourceInterface, p *v1alpha1.Pipeline) (*unstructured.Unstructured, error) {
	c, err := channelResourceInterface.Get(channelName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			newChannel, err := resources.NewChannel(channelName, p)
			logging.FromContext(ctx).Error(fmt.Sprintf("Creating Channel Object: %+v", newChannel))
			if err != nil {
				logging.FromContext(ctx).Error(fmt.Sprintf("Failed to create Channel resource object: %s/%s", p.Namespace, channelName), zap.Error(err))
				return nil, err
			}
			created, err := channelResourceInterface.Create(newChannel, metav1.CreateOptions{})
			if err != nil {
				logging.FromContext(ctx).Error(fmt.Sprintf("Failed to create Channel: %s/%s", p.Namespace, channelName), zap.Error(err))
				return nil, err
			}
			logging.FromContext(ctx).Error(fmt.Sprintf("Created Channel: %s/%s", p.Namespace, channelName), zap.Any("NewChannel", zap.Any("NewChannel", newChannel)))
			return created, nil
		} else {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to get Channel: %s/%s", p.Namespace, channelName), zap.Error(err))
			return nil, err
		}
	}
	logging.FromContext(ctx).Error(fmt.Sprintf("Found Channel: %s/%s", p.Namespace, channelName), zap.Any("NewChannel", c))
	return c, nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, step int, p *v1alpha1.Pipeline) (*eventingv1alpha1.Subscription, error) {
	expected := resources.NewSubscription(step, p)

	subName := resources.PipelineSubscriptionName(p.Name, step)
	sub, err := r.subscriptionLister.Subscriptions(p.Namespace).Get(subName)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		sub = expected
		logging.FromContext(ctx).Info(fmt.Sprintf("Creating subscription: %+v : SUBSCRIBERSPEC: %+v", sub, sub.Spec.Subscriber))
		newSub, err := r.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Create(sub)
		if err != nil {
			// TODO: Send events here, or elsewhere?
			//			r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Pipeline's subscription failed: %v", err)
			return nil, err
		}
		return newSub, nil
	} else if err != nil {
		logging.FromContext(ctx).Error("Failed to get subscription", zap.Error(err))
		// TODO: Send events here, or elsewhere?
		//		r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Pipelines's subscription failed: %v", err)
		return nil, err
	}
	return sub, nil
}
