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
	"errors"
	"fmt"

	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
	duckapis "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/resolver"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/broker"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/broker/resources"
	"knative.dev/eventing/pkg/reconciler/names"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the Broker reconciliation process.
	brokerReconcileError = "BrokerReconcileError"
	brokerReconciled     = "BrokerReconciled"

	// Label used to specify which Broker provides the implementation.
	brokerAnnotationKey = "eventing.knative.dev/broker.class"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	brokerLister       eventinglisters.BrokerLister
	serviceLister      corev1listers.ServiceLister
	deploymentLister   appsv1listers.DeploymentLister
	subscriptionLister messaginglisters.SubscriptionLister
	triggerLister      eventinglisters.TriggerLister

	channelableTracker duck.ListableTracker

	ingressImage              string
	ingressServiceAccountName string
	filterImage               string
	filterServiceAccountName  string

	// Dynamic tracker to track KResources. In particular, it tracks the dependency between Triggers and Sources.
	kresourceTracker duck.ListableTracker

	// Dynamic tracker to track AddressableTypes. In particular, it tracks Trigger subscribers.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver

	// If specified, only reconcile brokers with these labels
	brokerClass string
}

// Check that our Reconciler implements Interface
var _ brokerreconciler.Interface = (*Reconciler)(nil)
var _ brokerreconciler.Finalizer = (*Reconciler)(nil)

var brokerGVK = v1alpha1.SchemeGroupVersion.WithKind("Broker")

// ReconcilerArgs are the arguments needed to create a broker.Reconciler.
type ReconcilerArgs struct {
	IngressImage              string
	IngressServiceAccountName string
	FilterImage               string
	FilterServiceAccountName  string
}

func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, brokerReconciled, "Broker reconciled: \"%s/%s\"", namespace, name)
}

func (r *Reconciler) ReconcileKind(ctx context.Context, b *v1alpha1.Broker) pkgreconciler.Event {
	// TODO(vaikas): Can we just add this into the controller as part of the filtering
	// so they won't even get queued into my queue???
	if r.brokerClass != "" {
		if b.GetAnnotations()[brokerAnnotationKey] != r.brokerClass {
			logging.FromContext(ctx).Info("Not reconciling broker, cause it's not mine", zap.String("broker", b.Name))
			return nil
		}
	}
	filterSvc, err := r.reconcileKind(ctx, b)

	if b.Status.IsReady() {
		// So, at this point the Broker is ready and everything should be solid
		// for the triggers to act upon, so reconcile them.
		te := r.reconcileTriggers(ctx, b, filterSvc)
		if te != nil {
			logging.FromContext(ctx).Error("Problem reconciling triggers", zap.Error(te))
			return fmt.Errorf("failed to reconcile triggers: %v", err)
		}
	} else {
		// Broker is not ready, but propagate it's status to my triggers.
		if te := r.propagateBrokerStatusToTriggers(ctx, b.Namespace, b.Name, &b.Status); te != nil {
			return fmt.Errorf("Trigger reconcile failed: %v", te)
		}
	}
	return err
}

func (r *Reconciler) reconcileKind(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, pkgreconciler.Event) {
	logging.FromContext(ctx).Debug("Reconciling", zap.Any("Broker", b))
	b.Status.InitializeConditions()
	b.Status.ObservedGeneration = b.Generation

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
	if b.Spec.ChannelTemplate == nil {
		r.Logger.Error("Broker.Spec.ChannelTemplate is nil",
			zap.String("namespace", b.Namespace), zap.String("name", b.Name))
		return nil, errors.New("Broker.Spec.ChannelTemplate is nil")
	}

	gvr, _ := meta.UnsafeGuessKindToResource(b.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())
	channelResourceInterface := r.DynamicClientSet.Resource(gvr).Namespace(b.Namespace)
	if channelResourceInterface == nil {
		return nil, fmt.Errorf("unable to create dynamic client for: %+v", b.Spec.ChannelTemplate)
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
		return nil, fmt.Errorf("unable to track changes to the trigger Channel: %v", err)
	}

	logging.FromContext(ctx).Info("Reconciling the trigger channel")
	triggerChan, err := r.reconcileTriggerChannel(ctx, channelResourceInterface, triggerChannelObjRef, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the trigger channel", zap.Error(err))
		b.Status.MarkTriggerChannelFailed("ChannelFailure", "%v", err)
		return nil, fmt.Errorf("Failed to reconcile trigger channel: %v", err)
	}

	if triggerChan.Status.Address == nil {
		logging.FromContext(ctx).Debug("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		// Ok to return nil for error here, once channel address becomes available, this will get requeued.
		return nil, nil
	}
	if url := triggerChan.Status.Address.GetURL(); url.Host == "" {
		// We check the trigger Channel's address here because it is needed to create the Ingress Deployment.
		logging.FromContext(ctx).Debug("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		// Ok to return nil for error here, once channel address becomes available, this will get requeued.
		return nil, nil
	}
	b.Status.TriggerChannel = &triggerChannelObjRef
	b.Status.PropagateTriggerChannelReadiness(&triggerChan.Status)

	filterDeployment, err := r.reconcileFilterDeployment(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling filter Deployment", zap.Error(err))
		b.Status.MarkFilterFailed("DeploymentFailure", "%v", err)
		return nil, err
	}
	filterSvc, err := r.reconcileFilterService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling filter Service", zap.Error(err))
		b.Status.MarkFilterFailed("ServiceFailure", "%v", err)
		return nil, err
	}
	b.Status.PropagateFilterDeploymentAvailability(filterDeployment)

	ingressDeployment, err := r.reconcileIngressDeployment(ctx, b, triggerChan)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Deployment", zap.Error(err))
		b.Status.MarkIngressFailed("DeploymentFailure", "%v", err)
		return nil, err
	}

	svc, err := r.reconcileIngressService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Service", zap.Error(err))
		b.Status.MarkIngressFailed("ServiceFailure", "%v", err)
		return nil, err
	}
	b.Status.PropagateIngressDeploymentAvailability(ingressDeployment)
	b.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
	})

	// So, at this point the Broker is ready and everything should be solid
	// for the triggers to act upon.
	return filterSvc, nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, b *v1alpha1.Broker) pkgreconciler.Event {
	if err := r.propagateBrokerStatusToTriggers(ctx, b.Namespace, b.Name, nil); err != nil {
		return fmt.Errorf("Trigger reconcile failed: %v", err)
	}
	return newReconciledNormal(b.Namespace, b.Name)
}

// reconcileFilterDeployment reconciles Broker's 'b' filter deployment.
func (r *Reconciler) reconcileFilterDeployment(ctx context.Context, b *v1alpha1.Broker) (*v1.Deployment, error) {
	expected := resources.MakeFilterDeployment(&resources.FilterArgs{
		Broker:             b,
		Image:              r.filterImage,
		ServiceAccountName: r.filterServiceAccountName,
	})
	return r.reconcileDeployment(ctx, expected)
}

// reconcileFilterService reconciles Broker's 'b' filter service.
func (r *Reconciler) reconcileFilterService(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, error) {
	expected := resources.MakeFilterService(b)
	return r.reconcileService(ctx, expected)
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

// reconcileDeployment reconciles the K8s Deployment 'd'.
func (r *Reconciler) reconcileDeployment(ctx context.Context, d *v1.Deployment) (*v1.Deployment, error) {
	current, err := r.deploymentLister.Deployments(d.Namespace).Get(d.Name)
	if apierrs.IsNotFound(err) {
		current, err = r.KubeClientSet.AppsV1().Deployments(d.Namespace).Create(d)
		if err != nil {
			return nil, err
		}
		return current, nil
	} else if err != nil {
		return nil, err
	}

	if !equality.Semantic.DeepDerivative(d.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = d.Spec
		current, err = r.KubeClientSet.AppsV1().Deployments(current.Namespace).Update(desired)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// reconcileService reconciles the K8s Service 'svc'.
func (r *Reconciler) reconcileService(ctx context.Context, svc *corev1.Service) (*corev1.Service, error) {
	current, err := r.serviceLister.Services(svc.Namespace).Get(svc.Name)
	if apierrs.IsNotFound(err) {
		current, err = r.KubeClientSet.CoreV1().Services(svc.Namespace).Create(svc)
		if err != nil {
			return nil, err
		}
		return current, nil
	} else if err != nil {
		return nil, err
	}

	// spec.clusterIP is immutable and is set on existing services. If we don't set this to the same value, we will
	// encounter an error while updating.
	svc.Spec.ClusterIP = current.Spec.ClusterIP
	if !equality.Semantic.DeepDerivative(svc.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = svc.Spec
		current, err = r.KubeClientSet.CoreV1().Services(current.Namespace).Update(desired)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// reconcileIngressDeploymentCRD reconciles the Ingress Deployment for a CRD backed channel.
func (r *Reconciler) reconcileIngressDeployment(ctx context.Context, b *v1alpha1.Broker, c *duckv1alpha1.Channelable) (*v1.Deployment, error) {
	expected := resources.MakeIngressDeployment(&resources.IngressArgs{
		Broker:             b,
		Image:              r.ingressImage,
		ServiceAccountName: r.ingressServiceAccountName,
		ChannelAddress:     c.Status.Address.GetURL().Host,
	})
	return r.reconcileDeployment(ctx, expected)
}

// reconcileIngressService reconciles the Ingress Service.
func (r *Reconciler) reconcileIngressService(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, error) {
	expected := resources.MakeIngressService(b)
	return r.reconcileService(ctx, expected)
}

// reconcileTriggers reconciles the Triggers that are pointed to this broker
func (r *Reconciler) reconcileTriggers(ctx context.Context, b *v1alpha1.Broker, filterSvc *corev1.Service) error {

	// TODO: Figure out the labels stuff... If webhook does it, we can filter like this...
	// Find all the Triggers that have been labeled as belonging to me
	/*
		triggers, err := r.triggerLister.Triggers(b.Namespace).List(labels.SelectorFromSet(brokerLabels(b.brokerClass)))
	*/
	triggers, err := r.triggerLister.Triggers(b.Namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	for _, t := range triggers {
		if t.Spec.Broker == b.Name {
			trigger := t.DeepCopy()
			tErr := r.reconcileTrigger(ctx, b, trigger, filterSvc)
			if tErr != nil {
				r.Logger.Error("Reconciling trigger failed:", zap.String("name", t.Name), zap.Error(err))
				r.Recorder.Eventf(trigger, corev1.EventTypeWarning, triggerReconcileFailed, "Trigger reconcile failed: %v", tErr)
			} else {
				r.Recorder.Event(trigger, corev1.EventTypeNormal, triggerReconciled, "Trigger reconciled")
			}
			trigger.Status.ObservedGeneration = t.Generation
			if _, updateStatusErr := r.updateTriggerStatus(ctx, trigger); updateStatusErr != nil {
				logging.FromContext(ctx).Error("Failed to update Trigger status", zap.Error(updateStatusErr))
				r.Recorder.Eventf(trigger, corev1.EventTypeWarning, triggerUpdateStatusFailed, "Failed to update Trigger's status: %v", updateStatusErr)
			}
		}
	}
	return nil
}

/* TODO: Enable once we start filtering by classes of brokers
func brokerLabels(name string) map[string]string {
	return map[string]string{
		brokerAnnotationKey: name,
	}
}
*/

func (r *Reconciler) propagateBrokerStatusToTriggers(ctx context.Context, namespace, name string, bs *v1alpha1.BrokerStatus) error {
	triggers, err := r.triggerLister.Triggers(namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	for _, t := range triggers {
		if t.Spec.Broker == name {
			// Don't modify informers copy
			trigger := t.DeepCopy()
			trigger.Status.InitializeConditions()
			if bs == nil {
				trigger.Status.MarkBrokerFailed("BrokerDoesNotExist", "Broker %q does not exist", name)
			} else {
				trigger.Status.PropagateBrokerStatus(bs)
			}
			if _, updateStatusErr := r.updateTriggerStatus(ctx, trigger); updateStatusErr != nil {
				logging.FromContext(ctx).Error("Failed to update Trigger status", zap.Error(updateStatusErr))
				r.Recorder.Eventf(trigger, corev1.EventTypeWarning, triggerUpdateStatusFailed, "Failed to update Trigger's status: %v", updateStatusErr)
				return updateStatusErr
			}
		}
	}
	return nil
}
