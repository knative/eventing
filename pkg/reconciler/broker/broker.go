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

package broker

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	duckapis "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/broker/resources"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	brokerReadinessChanged          = "BrokerReadinessChanged"
	brokerReconcileError            = "BrokerReconcileError"
	brokerUpdateStatusFailed        = "BrokerUpdateStatusFailed"
	ingressSubscriptionDeleteFailed = "IngressSubscriptionDeleteFailed"
	ingressSubscriptionCreateFailed = "IngressSubscriptionCreateFailed"
	ingressSubscriptionGetFailed    = "IngressSubscriptionGetFailed"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	brokerLister       eventinglisters.BrokerLister
	subscriptionLister messaginglisters.SubscriptionLister

	channelableTracker duck.ListableTracker

	serviceHelper *reconciler.ServiceHelper

	ingressImage              string
	ingressServiceAccountName string
	filterImage               string
	filterServiceAccountName  string
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// ReconcilerArgs are the arguments needed to create a broker.Reconciler.
type ReconcilerArgs struct {
	IngressImage              string
	IngressServiceAccountName string
	FilterImage               string
	FilterServiceAccountName  string
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Broker resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the Broker resource with this namespace/name
	original, err := r.brokerLister.Brokers(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Info("broker key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	broker := original.DeepCopy()

	// Reconcile this copy of the Broker and then write back any status
	// updates regardless of whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, broker)
	if reconcileErr != nil {
		logging.FromContext(ctx).Warn("Error reconciling Broker", zap.Error(reconcileErr))
		r.Recorder.Eventf(broker, corev1.EventTypeWarning, brokerReconcileError, fmt.Sprintf("Broker reconcile error: %v", reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("Broker reconciled")
	}

	// Since the reconciler took a crack at this, make sure it's reflected
	// in the status correctly.
	broker.Status.ObservedGeneration = original.Generation

	if _, updateStatusErr := r.updateStatus(ctx, broker); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the Broker status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(broker, corev1.EventTypeWarning, brokerUpdateStatusFailed, "Failed to update Broker's status: %v", updateStatusErr)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, b *v1alpha1.Broker) error {
	logging.FromContext(ctx).Debug("Reconciling", zap.Any("Broker", b))
	b.Status.InitializeConditions()

	// 1. Trigger Channel is created for all events. Triggers will Subscribe to this Channel.
	// 2. Filter Deployment.
	// 3. Ingress Deployment.
	// 4. K8s Services that point at the Deployments.
	// 5. Ingress Channel is created to get events from Triggers back into this Broker via the
	//    Ingress Deployment.
	//   - Ideally this wouldn't exist and we would point the Trigger's reply directly to the K8s
	//     Service. However, Subscriptions only allow us to send replies to Channels, so we need
	//     this as an intermediary.
	// 6. Subscription from the Ingress Channel to the Ingress Service.

	if b.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	if b.Spec.ChannelTemplate == nil {
		r.Logger.Error("Broker.Spec.ChannelTemplate is nil",
			zap.String("namespace", b.Namespace), zap.String("name", b.Name))
		return nil
	}

	gvr, _ := meta.UnsafeGuessKindToResource(b.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())
	channelResourceInterface := r.DynamicClientSet.Resource(gvr).Namespace(b.Namespace)
	if channelResourceInterface == nil {
		return fmt.Errorf("unable to create dynamic client for: %+v", b.Spec.ChannelTemplate)
	}

	track := r.channelableTracker.TrackInNamespace(b)

	triggerChannelName := resources.BrokerChannelName(b.Name, "trigger")
	triggerChannelObjRef := corev1.ObjectReference{
		Kind:       b.Spec.ChannelTemplate.Kind,
		APIVersion: b.Spec.ChannelTemplate.APIVersion,
		Name:       triggerChannelName,
		Namespace:  b.Namespace,
	}
	// Start tracking the trigger channel.
	if err := track(triggerChannelObjRef); err != nil {
		return fmt.Errorf("unable to track changes to the trigger Channel: %v", err)
	}

	logging.FromContext(ctx).Info("Reconciling the trigger channel")
	triggerChan, err := r.reconcileTriggerChannel(ctx, channelResourceInterface, triggerChannelObjRef, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the trigger channel", zap.Error(err))
		b.Status.MarkTriggerChannelFailed("ChannelFailure", "%v", err)
		return err
	}

	if triggerChan.Status.Address == nil {
		logging.FromContext(ctx).Debug("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		return nil
	}
	if url := triggerChan.Status.Address.GetURL(); url.Host == "" {
		// We check the trigger Channel's address here because it is needed to create the Ingress Deployment.
		logging.FromContext(ctx).Debug("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		return nil
	}
	b.Status.TriggerChannel = &triggerChannelObjRef
	b.Status.PropagateTriggerChannelReadiness(&triggerChan.Status)

	filterStatus, err := r.reconcileFilterService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconcile filter service", zap.Error(err))
		b.Status.MarkFilterFailed("ServiceFailure", "%v", err)
		return err
	}
	if filterStatus.IsReady {
		b.Status.MarkFilterReady()
	} else {
		b.Status.MarkFilterFailed(filterStatus.Reason, filterStatus.Message)
	}

	ingressStatus, err := r.reconcileIngressService(ctx, b, triggerChan)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconcile ingress service", zap.Error(err))
		b.Status.MarkIngressFailed("ServiceFailure", "%v", err)
		return err
	}
	if ingressStatus.IsReady {
		b.Status.MarkIngressReady()
	} else {
		b.Status.MarkIngressFailed(ingressStatus.Reason, ingressStatus.Message)
	}

	b.Status.SetAddress(ingressStatus.URL)
	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Broker) (*v1alpha1.Broker, error) {
	broker, err := r.brokerLister.Brokers(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(broker.Status, desired.Status) {
		return broker, nil
	}

	becomesReady := desired.Status.IsReady() && !broker.Status.IsReady()

	// Don't modify the informers copy.
	existing := broker.DeepCopy()
	existing.Status = desired.Status

	b, err := r.EventingClientSet.EventingV1alpha1().Brokers(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(b.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Sugar().Infof("Broker %q became ready after %v", broker.Name, duration)
		r.Recorder.Event(broker, corev1.EventTypeNormal, brokerReadinessChanged, fmt.Sprintf("Broker %q became ready", broker.Name))
		if err := r.StatsReporter.ReportReady("Broker", broker.Namespace, broker.Name, duration); err != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for Broker, %v", err)
		}
	}

	return b, err
}

func (r *Reconciler) reconcileFilterService(ctx context.Context, b *v1alpha1.Broker) (*reconciler.ServiceStatus, error) {
	svcArgs := resources.MakeFilterServiceArgs(&resources.FilterArgs{
		Broker:             b,
		Image:              r.filterImage,
		ServiceAccountName: r.filterServiceAccountName,
	})
	return r.serviceHelper.ReconcileService(ctx, b, *svcArgs)
}

func (r *Reconciler) reconcileIngressService(ctx context.Context, b *v1alpha1.Broker, c *duckv1alpha1.Channelable) (*reconciler.ServiceStatus, error) {
	svcArgs := resources.MakeIngressServiceArgs(&resources.IngressArgs{
		Broker:             b,
		Image:              r.ingressImage,
		ServiceAccountName: r.ingressServiceAccountName,
		ChannelAddress:     c.Status.Address.GetURL().Host,
	})
	return r.serviceHelper.ReconcileService(ctx, b, *svcArgs)
}

func newTriggerChannel(b *v1alpha1.Broker) (*unstructured.Unstructured, error) {
	return resources.NewChannel("trigger", b, TriggerChannelLabels(b.Name))
}

func (r *Reconciler) reconcileTriggerChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, channelObjRef corev1.ObjectReference, b *v1alpha1.Broker) (*duckv1alpha1.Channelable, error) {
	c, err := newTriggerChannel(b)
	if err != nil {
		logging.FromContext(ctx).Error(fmt.Sprintf("Failed to create Trigger Channel object: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err
	}
	return r.reconcileChannel(ctx, channelResourceInterface, channelObjRef, c, b)
}

// reconcileChannel reconciles Broker's 'b' underlying channel.
func (r *Reconciler) reconcileChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, channelObjRef corev1.ObjectReference, newChannel *unstructured.Unstructured, b *v1alpha1.Broker) (*duckv1alpha1.Channelable, error) {
	lister, err := r.channelableTracker.ListerFor(channelObjRef)
	if err != nil {
		logging.FromContext(ctx).Error(fmt.Sprintf("Error getting lister for Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err
	}
	c, err := lister.ByNamespace(channelObjRef.Namespace).Get(channelObjRef.Name)
	// If the resource doesn't exist, we'll create it
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Debug(fmt.Sprintf("Creating Channel Object: %+v", newChannel))
			created, err := channelResourceInterface.Create(newChannel, metav1.CreateOptions{})
			if err != nil {
				logging.FromContext(ctx).Error(fmt.Sprintf("Failed to create Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
				return nil, err
			}
			logging.FromContext(ctx).Info(fmt.Sprintf("Created Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Any("NewChannel", newChannel))
			channelable := &duckv1alpha1.Channelable{}
			err = duckapis.FromUnstructured(created, channelable)
			if err != nil {
				logging.FromContext(ctx).Error(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Any("createdChannel", created), zap.Error(err))
				return nil, err

			}
			return channelable, nil
		}
		logging.FromContext(ctx).Error(fmt.Sprintf("Failed to get Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err
	}
	logging.FromContext(ctx).Debug(fmt.Sprintf("Found Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name))
	channelable, ok := c.(*duckv1alpha1.Channelable)
	if !ok {
		logging.FromContext(ctx).Error(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err
	}
	return channelable, nil
}

// TriggerChannelLabels are all the labels placed on the Trigger Channel for the given brokerName. This
// should only be used by Broker and Trigger code.
func TriggerChannelLabels(brokerName string) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":           brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
}
