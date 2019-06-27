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

	"knative.dev/pkg/kmeta"

	duckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventinglisters "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/broker/resources"
	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/eventing/pkg/utils"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	duckroot "knative.dev/pkg/apis"
	duckapis "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	brokerReadinessChanged          = "BrokerReadinessChanged"
	brokerReconcileError            = "BrokerReconcileError"
	brokerUpdateStatusFailed        = "BrokerUpdateStatusFailed"
	ingressSubscriptionDeleteFailed = "IngressSubscriptionDeleteFailed"
	ingressSubscriptionCreateFailed = "IngressSubscriptionCreateFailed"
	deprecatedMessage               = "Provisioners are deprecated and will be removed in 0.8. Recommended replacement is CRD based channels using spec.channelTemplateSpec."
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	brokerLister       eventinglisters.BrokerLister
	channelLister      eventinglisters.ChannelLister
	serviceLister      corev1listers.ServiceLister
	deploymentLister   appsv1listers.DeploymentLister
	subscriptionLister eventinglisters.SubscriptionLister

	addressableTracker duck.AddressableTracker

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
	if b.Spec.ChannelTemplate.Kind != "" && b.Spec.ChannelTemplate.APIVersion != "" {
		return r.reconcileCRD(ctx, b)
	} else {
		return r.reconcileLegacy(ctx, b)
	}
}

// reconcileLegacy reconciles Provisioner based Brokers.
func (r *Reconciler) reconcileLegacy(ctx context.Context, b *v1alpha1.Broker) error {
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

	b.Status.MarkDeprecated("ClusterChannelProvisionerDeprecated", deprecatedMessage)

	triggerChan, err := r.reconcileTriggerChannel(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the trigger channel", zap.Error(err))
		b.Status.MarkTriggerChannelFailed("ChannelFailure", "%v", err)
		return err
	} else if url := triggerChan.Status.Address.GetURL(); url.Host == "" {
		// We check the trigger Channel's address here because it is needed to create the Ingress
		// Deployment.
		logging.FromContext(ctx).Debug("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		return nil
	}
	b.Status.TriggerChannel = &corev1.ObjectReference{
		Kind:       "Channel",
		APIVersion: "eventing.knative.dev/v1alpha1",
		Name:       triggerChan.Name,
		Namespace:  triggerChan.Namespace,
	}
	b.Status.PropagateTriggerChannelReadiness(&triggerChan.Status)

	filterDeployment, err := r.reconcileFilterDeployment(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling filter Deployment", zap.Error(err))
		b.Status.MarkFilterFailed("DeploymentFailure", "%v", err)
		return err
	}
	_, err = r.reconcileFilterService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling filter Service", zap.Error(err))
		b.Status.MarkFilterFailed("ServiceFailure", "%v", err)
		return err
	}
	b.Status.PropagateFilterDeploymentAvailability(filterDeployment)

	ingressDeployment, err := r.reconcileIngressDeployment(ctx, b, triggerChan)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Deployment", zap.Error(err))
		b.Status.MarkIngressFailed("DeploymentFailure", "%v", err)
		return err
	}

	svc, err := r.reconcileIngressService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Service", zap.Error(err))
		b.Status.MarkIngressFailed("ServiceFailure", "%v", err)
		return err
	}
	b.Status.PropagateIngressDeploymentAvailability(ingressDeployment)
	b.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
	})

	ingressChan, err := r.reconcileIngressChannel(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the ingress channel", zap.Error(err))
		b.Status.MarkIngressChannelFailed("ChannelFailure", "%v", err)
		return err
	}
	b.Status.IngressChannel = &corev1.ObjectReference{
		Kind:       "Channel",
		APIVersion: "eventing.knative.dev/v1alpha1",
		Name:       ingressChan.Name,
		Namespace:  ingressChan.Namespace,
	}
	b.Status.PropagateIngressChannelReadiness(&ingressChan.Status)

	ingressSub, err := r.reconcileIngressSubscription(ctx, b, ingressChan, svc)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the ingress subscription", zap.Error(err))
		b.Status.MarkIngressSubscriptionFailed("SubscriptionFailure", "%v", err)
		return err
	}
	b.Status.PropagateIngressSubscriptionReadiness(&ingressSub.Status)

	return nil
}

// reconcileCRD reconciles channel CRD based implementations.
func (r *Reconciler) reconcileCRD(ctx context.Context, b *v1alpha1.Broker) error {
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

	channelResourceInterface := r.DynamicClientSet.Resource(duckroot.KindToResource(b.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())).Namespace(b.Namespace)

	triggerChannelName := resources.BrokerChannelName(b.Name, "trigger")
	logging.FromContext(ctx).Info("Reconciling the trigger channel CRD")
	triggerChan, err := r.reconcileTriggerChannelCRD(ctx, triggerChannelName, channelResourceInterface, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the trigger channel", zap.Error(err))
		b.Status.MarkTriggerChannelFailed("ChannelFailure", "%v", err)
		return err
	}

	// Tell tracker to reconcile this Broker whenever my channels change.
	track := r.addressableTracker.TrackInNamespace(b)

	// Start tracking trigger channel...
	if err = track(utils.ObjectRef(triggerChan, triggerChan.GroupVersionKind())); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to Channel", zap.Error(err))
		return err
	}

	if triggerChan.Status.Address == nil {
		logging.FromContext(ctx).Debug("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		return nil
	}
	if url := triggerChan.Status.Address.GetURL(); url.Host == "" {
		// We check the trigger Channel's address here because it is needed to create the Ingress
		// Deployment.
		logging.FromContext(ctx).Debug("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		return nil
	}
	b.Status.TriggerChannel = &corev1.ObjectReference{
		Kind:       triggerChan.Kind,
		APIVersion: triggerChan.APIVersion,
		Name:       triggerChan.Name,
		Namespace:  triggerChan.Namespace,
	}
	b.Status.PropagateTriggerChannelReadinessCRD(&triggerChan.Status)

	filterDeployment, err := r.reconcileFilterDeployment(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling filter Deployment", zap.Error(err))
		b.Status.MarkFilterFailed("DeploymentFailure", "%v", err)
		return err
	}
	_, err = r.reconcileFilterService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling filter Service", zap.Error(err))
		b.Status.MarkFilterFailed("ServiceFailure", "%v", err)
		return err
	}
	b.Status.PropagateFilterDeploymentAvailability(filterDeployment)

	ingressDeployment, err := r.reconcileIngressDeploymentCRD(ctx, b, triggerChan)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Deployment", zap.Error(err))
		b.Status.MarkIngressFailed("DeploymentFailure", "%v", err)
		return err
	}

	svc, err := r.reconcileIngressService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Service", zap.Error(err))
		b.Status.MarkIngressFailed("ServiceFailure", "%v", err)
		return err
	}
	b.Status.PropagateIngressDeploymentAvailability(ingressDeployment)
	b.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
	})

	ingressChannelName := resources.BrokerChannelName(b.Name, "ingress")
	ingressChan, err := r.reconcileIngressChannelCRD(ctx, ingressChannelName, channelResourceInterface, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the ingress channel", zap.Error(err))
		b.Status.MarkIngressChannelFailed("ChannelFailure", "%v", err)
		return err
	}
	b.Status.IngressChannel = &corev1.ObjectReference{
		Kind:       ingressChan.Kind,
		APIVersion: ingressChan.APIVersion,
		Name:       ingressChan.Name,
		Namespace:  ingressChan.Namespace,
	}
	b.Status.PropagateIngressChannelReadinessCRD(&ingressChan.Status)

	// Start tracking ingress channel...
	if err = track(utils.ObjectRef(ingressChan, ingressChan.GroupVersionKind())); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to Channel", zap.Error(err))
		return err
	}

	ingressSub, err := r.reconcileIngressSubscriptionCRD(ctx, b, ingressChan, svc)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the ingress subscription", zap.Error(err))
		b.Status.MarkIngressSubscriptionFailed("SubscriptionFailure", "%v", err)
		return err
	}
	b.Status.PropagateIngressSubscriptionReadiness(&ingressSub.Status)

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

func (r *Reconciler) reconcileTriggerChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	get := func() (*v1alpha1.Channel, error) {
		return r.getChannel(ctx, b, labels.SelectorFromSet(TriggerChannelLabels(b.Name)))
	}
	return r.reconcileChannel(ctx, get, newTriggerChannel(b))
}

func (r *Reconciler) reconcileIngressChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	get := func() (*v1alpha1.Channel, error) {
		return r.getChannel(ctx, b, labels.SelectorFromSet(IngressChannelLabels(b.Name)))
	}
	return r.reconcileChannel(ctx, get, newIngressChannel(b))
}

// reconcileChannel reconciles Broker's 'b' underlying channel.
func (r *Reconciler) reconcileChannel(ctx context.Context, get func() (*v1alpha1.Channel, error), newChan *v1alpha1.Channel) (*v1alpha1.Channel, error) {
	c, err := get()
	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		c, err = r.EventingClientSet.EventingV1alpha1().Channels(newChan.Namespace).Create(newChan)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else if err != nil {
		return nil, err
	}

	// TODO Determine if we want to update spec (maybe just args?). For now, do not update it.

	return c, nil
}

// getChannel returns the Channel object for Broker 'b' if exists, otherwise it returns an error.
func (r *Reconciler) getChannel(ctx context.Context, b *v1alpha1.Broker, ls labels.Selector) (*v1alpha1.Channel, error) {
	channels, err := r.channelLister.Channels(b.Namespace).List(ls)
	if err != nil {
		return nil, err
	}
	for _, c := range channels {
		if metav1.IsControlledBy(c, b) {
			return c, nil
		}
	}

	return nil, apierrs.NewNotFound(schema.GroupResource{}, "")
}

func newTriggerChannel(b *v1alpha1.Broker) *v1alpha1.Channel {
	return newChannel("trigger", b, TriggerChannelLabels(b.Name))
}

func newIngressChannel(b *v1alpha1.Broker) *v1alpha1.Channel {
	return newChannel("ingress", b, IngressChannelLabels(b.Name))
}

func newTriggerChannelCRD(b *v1alpha1.Broker) (*unstructured.Unstructured, error) {
	return resources.NewChannel("trigger", b, TriggerChannelLabels(b.Name))
}

func newIngressChannelCRD(b *v1alpha1.Broker) (*unstructured.Unstructured, error) {
	return resources.NewChannel("ingress", b, IngressChannelLabels(b.Name))
}

// newChannel creates a new Channel for Broker 'b'.
func newChannel(channelType string, b *v1alpha1.Broker, l map[string]string) *v1alpha1.Channel {
	var spec v1alpha1.ChannelSpec
	if b.Spec.DeprecatedChannelTemplate != nil {
		spec = *b.Spec.DeprecatedChannelTemplate
	}

	return &v1alpha1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Channel",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.Namespace,
			Name:      resources.BrokerChannelName(b.Name, channelType),
			Labels:    l,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(b),
			},
		},
		Spec: spec,
	}
}

func (r *Reconciler) reconcileTriggerChannelCRD(ctx context.Context, channelName string, channelResourceInterface dynamic.ResourceInterface, b *v1alpha1.Broker) (*duckv1alpha1.Channelable, error) {
	c, err := newTriggerChannelCRD(b)
	if err != nil {
		logging.FromContext(ctx).Error(fmt.Sprintf("Failed to create Trigger Channel CRD object: %s/%s", b.Namespace, channelName), zap.Error(err))
		return nil, err
	}
	return r.reconcileChannelCRD(ctx, channelName, channelResourceInterface, c, b)
}

func (r *Reconciler) reconcileIngressChannelCRD(ctx context.Context, channelName string, channelResourceInterface dynamic.ResourceInterface, b *v1alpha1.Broker) (*duckv1alpha1.Channelable, error) {
	c, err := newIngressChannelCRD(b)
	if err != nil {
		return nil, err
	}
	return r.reconcileChannelCRD(ctx, channelName, channelResourceInterface, c, b)
}

// reconcileChannelCRD reconciles Broker's 'b' underlying channel for CRD based Channels
func (r *Reconciler) reconcileChannelCRD(ctx context.Context, channelName string, channelResourceInterface dynamic.ResourceInterface, newChannel *unstructured.Unstructured, b *v1alpha1.Broker) (*duckv1alpha1.Channelable, error) {
	c, err := channelResourceInterface.Get(channelName, metav1.GetOptions{})
	// If the resource doesn't exist, we'll create it
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Debug(fmt.Sprintf("Creating Channel CRD Object: %+v", newChannel))
			created, err := channelResourceInterface.Create(newChannel, metav1.CreateOptions{})
			if err != nil {
				logging.FromContext(ctx).Error(fmt.Sprintf("Failed to create Channel: %s/%s", b.Namespace, channelName), zap.Error(err))
				return nil, err
			}
			logging.FromContext(ctx).Info(fmt.Sprintf("Created Channel: %s/%s", b.Namespace, channelName), zap.Any("NewChannel", newChannel))
			channelable := &duckv1alpha1.Channelable{}
			err = duckapis.FromUnstructured(created, channelable)
			if err != nil {
				logging.FromContext(ctx).Error(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", b.Namespace, channelName), zap.Any("createdChannel", created), zap.Error(err))
				return nil, err

			}
			return channelable, nil
		} else {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to get Channel: %s/%s", b.Namespace, channelName), zap.Error(err))
			return nil, err
		}
	}
	logging.FromContext(ctx).Debug(fmt.Sprintf("Found Channel: %s/%s", b.Namespace, channelName), zap.Any("NewChannel", c))
	channelable := &duckv1alpha1.Channelable{}
	err = duckapis.FromUnstructured(c, channelable)
	if err != nil {
		logging.FromContext(ctx).Error(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", b.Namespace, channelName), zap.Error(err))
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

// IngressChannelLabels are all the labels placed on the Ingress Channel for the given brokerName. This
// should only be used by Broker and Trigger code.
func IngressChannelLabels(brokerName string) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":        brokerName,
		"eventing.knative.dev/brokerIngress": "true",
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

// reconcileIngressDeployment reconciles the Ingress Deployment.
func (r *Reconciler) reconcileIngressDeployment(ctx context.Context, b *v1alpha1.Broker, c *v1alpha1.Channel) (*v1.Deployment, error) {
	expected := resources.MakeIngress(&resources.IngressArgs{
		Broker:             b,
		Image:              r.ingressImage,
		ServiceAccountName: r.ingressServiceAccountName,
		ChannelAddress:     c.Status.Address.GetURL().Host,
	})
	return r.reconcileDeployment(ctx, expected)
}

// reconcileIngressDeploymentCRD reconciles the Ingress Deployment for a CRD backed channel.
func (r *Reconciler) reconcileIngressDeploymentCRD(ctx context.Context, b *v1alpha1.Broker, c *duckv1alpha1.Channelable) (*v1.Deployment, error) {
	expected := resources.MakeIngress(&resources.IngressArgs{
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

func (r *Reconciler) reconcileIngressSubscription(ctx context.Context, b *v1alpha1.Broker, c *v1alpha1.Channel, svc *corev1.Service) (*v1alpha1.Subscription, error) {
	expected := makeSubscription(b, c, svc)

	sub, err := r.getIngressSubscription(ctx, b)
	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		sub, err = r.EventingClientSet.EventingV1alpha1().Subscriptions(expected.Namespace).Create(expected)
		if err != nil {
			return nil, err
		}
		return sub, nil
	} else if err != nil {
		return nil, err
	}

	// Update Subscription if it has changed. Ignore the generation.
	expected.Spec.DeprecatedGeneration = sub.Spec.DeprecatedGeneration
	if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		// Given that spec.channel is immutable, we cannot just update the subscription. We delete
		// it instead, and re-create it.
		err = r.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{})
		if err != nil {
			logging.FromContext(ctx).Info("Cannot delete subscription", zap.Error(err))
			r.Recorder.Eventf(b, corev1.EventTypeWarning, ingressSubscriptionDeleteFailed, "Delete Broker Ingress' subscription failed: %v", err)
			return nil, err
		}
		sub, err = r.EventingClientSet.EventingV1alpha1().Subscriptions(expected.Namespace).Create(expected)
		if err != nil {
			logging.FromContext(ctx).Info("Cannot create subscription", zap.Error(err))
			r.Recorder.Eventf(b, corev1.EventTypeWarning, ingressSubscriptionCreateFailed, "Create Broker Ingress' subscription failed: %v", err)
			return nil, err
		}
	}
	return sub, nil
}

func (r *Reconciler) reconcileIngressSubscriptionCRD(ctx context.Context, b *v1alpha1.Broker, c *duckv1alpha1.Channelable, svc *corev1.Service) (*v1alpha1.Subscription, error) {
	expected := resources.MakeSubscriptionCRD(b, c, svc)

	sub, err := r.getIngressSubscription(ctx, b)
	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		sub, err = r.EventingClientSet.EventingV1alpha1().Subscriptions(expected.Namespace).Create(expected)
		if err != nil {
			return nil, err
		}
		return sub, nil
	} else if err != nil {
		return nil, err
	}

	// Update Subscription if it has changed. Ignore the generation.
	expected.Spec.DeprecatedGeneration = sub.Spec.DeprecatedGeneration
	if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		// Given that spec.channel is immutable, we cannot just update the subscription. We delete
		// it instead, and re-create it.
		err = r.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{})
		if err != nil {
			logging.FromContext(ctx).Info("Cannot delete subscription", zap.Error(err))
			r.Recorder.Eventf(b, corev1.EventTypeWarning, ingressSubscriptionDeleteFailed, "Delete Broker Ingress' subscription failed: %v", err)
			return nil, err
		}
		sub, err = r.EventingClientSet.EventingV1alpha1().Subscriptions(expected.Namespace).Create(expected)
		if err != nil {
			logging.FromContext(ctx).Info("Cannot create subscription", zap.Error(err))
			r.Recorder.Eventf(b, corev1.EventTypeWarning, ingressSubscriptionCreateFailed, "Create Broker Ingress' subscription failed: %v", err)
			return nil, err
		}
	}
	return sub, nil
}

// getSubscription returns the first subscription controlled by Broker b
// otherwise it returns an error.
func (r *Reconciler) getIngressSubscription(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Subscription, error) {
	subscriptions, err := r.subscriptionLister.Subscriptions(b.Namespace).List(labels.SelectorFromSet(ingressSubscriptionLabels(b.Name)))
	if err != nil {
		return nil, err
	}
	for _, s := range subscriptions {
		if metav1.IsControlledBy(s, b) {
			return s, nil
		}
	}

	return nil, apierrs.NewNotFound(schema.GroupResource{}, "")
}

// makeSubscription returns a placeholder subscription for trigger 't', channel 'c', and service 'svc'.
func makeSubscription(b *v1alpha1.Broker, c *v1alpha1.Channel, svc *corev1.Service) *v1alpha1.Subscription {
	return &v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    b.Namespace,
			GenerateName: fmt.Sprintf("internal-ingress-%s-", b.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(b),
			},
			Labels: ingressSubscriptionLabels(b.Name),
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "Channel",
				Name:       c.Name,
			},
			Subscriber: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       svc.Name,
				},
			},
		},
	}
}

func ingressSubscriptionLabels(brokerName string) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":        brokerName,
		"eventing.knative.dev/brokerIngress": "true",
	}
}
